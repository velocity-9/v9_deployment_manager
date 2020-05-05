package deployment

import (
	"errors"
	"math/rand"
	"sync"
	"v9_deployment_manager/activator"
	"v9_deployment_manager/database"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"
)

const headHashSentinel = "HEAD"
const updaterChanSize = 1024

type HashAndInstanceCount struct {
	hash          string
	instanceCount int
}

type PathAndInstanceCount struct {
	path          worker.ComponentPath
	instanceCount int
}

type ActionManager struct {
	driver *database.Driver

	activator *activator.Activator
	workers   []*worker.V9Worker

	pathHashMux          sync.Mutex
	pathHashes           map[worker.ComponentPath]*HashAndInstanceCount
	pathHashUpdater      chan worker.ComponentID
	instanceCountUpdater chan PathAndInstanceCount

	dirtyStateNotifier chan struct{}
}

func NewActionManager(activator *activator.Activator, dr *database.Driver, workers []*worker.V9Worker) *ActionManager {
	pathHashes := make(map[worker.ComponentPath]*HashAndInstanceCount)

	pathHashUpdater := make(chan worker.ComponentID, updaterChanSize)
	instanceCountUpdater := make(chan PathAndInstanceCount, updaterChanSize)
	dirtyStateNotifier := make(chan struct{}, 1)

	mgr := &ActionManager{
		driver: dr,

		activator: activator,
		workers:   workers,

		pathHashes:           pathHashes,
		pathHashUpdater:      pathHashUpdater,
		instanceCountUpdater: instanceCountUpdater,

		dirtyStateNotifier: dirtyStateNotifier,
	}
	//Populate PathHashes with current system state
	mgr.pathHashMux.Lock()
	defer mgr.pathHashMux.Unlock()
	err := mgr.PopulatePathHashes()
	if err != nil {
		log.Error.Println("Could not get current system state")
	}

	//Thread for handling hash changes
	go func() {
		for {
			updatedID := <-pathHashUpdater
			path := worker.ComponentPath{
				User: updatedID.User,
				Repo: updatedID.Repo,
			}

			mgr.pathHashMux.Lock()
			mgr.pathHashes[path].hash = updatedID.Hash
			mgr.pathHashMux.Unlock()

			mgr.NotifyComponentStateChanged()
		}
	}()
	//Thread for handling instance count changes
	go func() {
		for {
			updatedID := <-instanceCountUpdater
			path := updatedID.path

			mgr.pathHashMux.Lock()
			log.Info.Println("component pathHash:", mgr.pathHashes[path])
			mgr.pathHashes[path].instanceCount = updatedID.instanceCount
			mgr.pathHashMux.Unlock()

			mgr.NotifyComponentStateChanged()
		}
	}()

	go func() {
		for {
			// Whenever we get a dirty state notification
			<-mgr.dirtyStateNotifier
			err := mgr.HandleDirtyState()
			if err != nil {
				log.Error.Println("Could not manage components:", err)
			}
		}
	}()

	return mgr
}

func (mgr *ActionManager) PopulatePathHashes() error {
	compMap := getCurrentInstanceState(mgr.workers)
	for compID, compStats := range compMap {
		tmp := &HashAndInstanceCount{compID.Hash, compStats.instanceCount}
		mgr.pathHashes[worker.ComponentPath{
			User: compID.User,
			Repo: compID.Repo,
		}] = tmp
	}

	return nil
}

func (mgr *ActionManager) NotifyComponentStateChanged() {
	// Put something in the `dirtyStateNotifier` -- unless someone else already notified that the state was dirty
	select {
	case mgr.dirtyStateNotifier <- struct{}{}:
	default:
	}
}

func (mgr *ActionManager) UpdateComponentHash(compID worker.ComponentID) {
	mgr.pathHashUpdater <- compID
}

func (mgr *ActionManager) UpdateInstanceCount(compPath worker.ComponentPath, instanceCount int) {
	mgr.instanceCountUpdater <- PathAndInstanceCount{compPath, instanceCount}
}

func (mgr *ActionManager) HandleDirtyState() error {
	// TODO: Parallelize this step (it basically single threads the deployment manager at the moment)

	// TODO: Smarter error handling

	// Lock the component hashes in place
	mgr.pathHashMux.Lock()
	defer mgr.pathHashMux.Unlock()

	log.Info.Println("Beginning dirty state handling")

	active, err := mgr.driver.FindActiveComponents()
	if err != nil {
		return err
	}

	// deactivate things that should not be running anywhere
	log.Info.Println("Deactivating non-active components")
	for _, w := range mgr.workers {
		err = mgr.deactivateNonactive(w, active)
		if err != nil {
			return err
		}
	}

	// start things that should be running somewhere but are not
	log.Info.Println("Starting active but not running components")
	for _, activeComp := range active {
		var hashToDeploy = headHashSentinel
		if mapVal, ok := mgr.pathHashes[activeComp]; ok {
			hashToDeploy = mapVal.hash
		}

		err = mgr.activateMissing(worker.ComponentID{
			User: activeComp.User,
			Repo: activeComp.Repo,
			Hash: hashToDeploy,
		})
		if err != nil {
			return err
		}
	}

	// ensure that, for each component, there is a worker running the latest version
	log.Info.Println("Ensuring that every component has the latest version running somewhere")
	for _, activeComp := range active {
		// We only need to make sure things are up to date when we know what's supposed to be running
		if correctHash, ok := mgr.pathHashes[activeComp]; ok {
			correctCompID := worker.ComponentID{
				User: activeComp.User,
				Repo: activeComp.Repo,
				Hash: correctHash.hash,
			}
			err = mgr.ensureNWorkerIsRunning(correctCompID)
			if err != nil {
				return err
			}
		}
	}

	// deactivate workers running old hashes of components
	log.Info.Println("Deactivating old hashes wherever they are")
	for _, activeComp := range active {
		correctHash, ok := mgr.pathHashes[activeComp]
		// If we couldn't grab the correct hash, whatever -- assume we're chugging along fine
		if !ok {
			continue
		}

		correctCompID := worker.ComponentID{
			User: activeComp.User,
			Repo: activeComp.Repo,
			Hash: correctHash.hash,
		}
		for _, w := range mgr.workers {
			err = mgr.deactivateIfHashDiffers(w, correctCompID)
			if err != nil {
				return err
			}
		}
	}

	// Scale up or down as requested by autoscaler
	// TODO this is probably in the wrong place
	log.Info.Println("Finished dirty state handling")
	return nil
}

func (mgr *ActionManager) deactivateNonactive(w *worker.V9Worker, active []worker.ComponentPath) error {
	status, err := w.Status()
	if err != nil {
		return err
	}

	nonActive := status.FindNonactive(active)
	for _, incorrectlyRunning := range nonActive {
		log.Info.Println("Deactivating incorrectly running", incorrectlyRunning, "on worker", w.URL)

		err := mgr.activator.Deactivate(incorrectlyRunning, w)
		if err != nil {
			return err
		}
	}

	return nil
}

func (mgr *ActionManager) activateMissing(toCheck worker.ComponentID) error {
	path := worker.ComponentPath{
		User: toCheck.User,
		Repo: toCheck.Repo,
	}

	for _, w := range mgr.workers {
		status, err := w.Status()
		if err != nil {
			return err
		}

		// If it is running somewhere we are good
		if status.ContainsPath(path) {
			return nil
		}
	}

	// Otherwise pick a worker randomly and deploy there
	randomWorker := mgr.workers[rand.Intn(len(mgr.workers))]
	log.Info.Println("Activating missing", toCheck, "on worker", randomWorker)
	activatedHash, err := mgr.activator.Activate(toCheck, randomWorker)
	if err != nil {
		return err
	}

	// Update the relevant hash (if we're using HEAD) so the map will match in the update step
	if toCheck.Hash == headHashSentinel {
		mgr.pathHashes[worker.ComponentPath{
			User: toCheck.User,
			Repo: toCheck.Repo,
		}].hash = activatedHash
	}

	return nil
}

func (mgr *ActionManager) ensureNWorkerIsRunning(compID worker.ComponentID) error {
	compPath := worker.ComponentPath{
		User: compID.User,
		Repo: compID.Repo,
	}

	//Get instance states
	compMap := getCurrentInstanceState(mgr.workers)
	// Check if the component is deployed on anything
	if _, ok := compMap[compID]; ok {
		//Check if should scale up current state instance count vs intended count
		if compMap[compID].instanceCount < mgr.pathHashes[compPath].instanceCount {
			log.Info.Println("SCALING UP:", compID)
			//Find worker where this comp isn't deployed
			workerToDeployTo, err := mgr.findWorkerToDeployTo(compID)
			if err != nil {
				log.Error.Println("Worker deployed on all nodes")
				return nil
			}
			//Deploy component and update hash
			err = mgr.deployToWorkerAndUpdateHash(workerToDeployTo, compID)
			if err != nil {
				return err
			}
			return nil
		}
		//Check if should scale down intended count vs actual count
		if compMap[compID].instanceCount > mgr.pathHashes[compPath].instanceCount {
			//Deactivate component on some worker
			log.Info.Println("SCALING DOWN:", compID)
			err := mgr.deactivateComponentOnSomeWorker(compID)
			if err != nil {
				return err
			}
			return nil
		}
	}
	//First time deploy
	//Deploy component to a random worker
	err := mgr.deployToWorkerAndUpdateHash(mgr.workers[rand.Intn(len(mgr.workers))], compID)
	if err != nil {
		return err
	}
	mgr.pathHashes[compPath] = &HashAndInstanceCount{compID.Hash, 1}
	return nil
}

func (mgr *ActionManager) deployToWorkerAndUpdateHash(w *worker.V9Worker, compID worker.ComponentID) error {
	compPath := worker.ComponentPath{
		User: compID.User,
		Repo: compID.Repo,
	}
	deployedHash, err := mgr.activator.Activate(compID, w)
	if err != nil {
		return err
	}

	//If this comp isn't in the map add it
	if _, ok := mgr.pathHashes[compPath]; ok {
		// Update the hash we're storing if we had HEAD
		if compID.Hash == headHashSentinel {
			mgr.pathHashes[compPath].hash = deployedHash
		}
	} else {
		tmp := &HashAndInstanceCount{deployedHash, 1}
		mgr.pathHashes[compPath] = tmp
	}
	return nil
}

func (mgr *ActionManager) deactivateComponentOnSomeWorker(compID worker.ComponentID) error {
	//Find worker where this comp is deployed
	workerToDeactivateOn, err := mgr.findWorkerToDeactivateOn(compID)
	if err != nil {
		log.Error.Println("Comp not running on any workers")
		return nil
	}
	//Deactivate
	err = mgr.activator.Deactivate(compID, workerToDeactivateOn)
	if err != nil {
		return err
	}
	return nil
}

func (mgr *ActionManager) findWorkerToDeployTo(compID worker.ComponentID) (*worker.V9Worker, error) {
	for _, w := range mgr.workers {
		status, err := w.Status()
		if err != nil {
			return nil, err
		}
		compIsOnWorker := false
		for _, runningComp := range status.ActiveComponents {
			if runningComp.ID == compID {
				compIsOnWorker = true
				break
			}
		}
		if !compIsOnWorker {
			return w, nil
		}
	}
	err := errors.New("comp on every worker")
	return nil, err
}

func (mgr *ActionManager) findWorkerToDeactivateOn(compID worker.ComponentID) (*worker.V9Worker, error) {
	for _, w := range mgr.workers {
		status, err := w.Status()
		if err != nil {
			return nil, err
		}
		compIsOnWorker := false
		for _, runningComp := range status.ActiveComponents {
			if runningComp.ID == compID {
				compIsOnWorker = true
				break
			}
		}
		if compIsOnWorker {
			return w, nil
		}
	}
	err := errors.New("comp on no workers")
	return nil, err
}

func (mgr *ActionManager) deactivateIfHashDiffers(w *worker.V9Worker, compID worker.ComponentID) error {
	status, err := w.Status()
	if err != nil {
		return err
	}

	for _, runningComp := range status.ActiveComponents {
		if runningComp.ID.User == compID.User && runningComp.ID.Repo == compID.Repo && runningComp.ID.Hash != compID.Hash {
			log.Info.Println("Doing to deactivate to ensure", w.URL, "does not keep running", compID)
			err = mgr.activator.Deactivate(runningComp.ID, w)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

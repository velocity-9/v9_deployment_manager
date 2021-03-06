package activator

import (
	"os"

	guuid "github.com/google/uuid"

	"v9_deployment_manager/database"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"
)

type Activator struct {
	driver *database.Driver
}

func CreateActivator(driver *database.Driver) *Activator {
	return &Activator{
		driver: driver,
	}
}

func (a *Activator) Activate(compID worker.ComponentID, worker *worker.V9Worker) (string, error) {
	// Setup the DB deploying entry
	err := a.driver.EnterDeploymentEntry(compID)
	if err != nil {
		log.Error.Println("Error starting deploy using db:", err)
		return "", err
	}
	defer func() {
		purgeErr := a.driver.PurgeDeploymentEntry(compID)
		if purgeErr != nil {
			log.Error.Println("Error purging deployment entry:", purgeErr)
		}
	}()

	// Get random tar name
	tarName := guuid.New().String()
	//Checkout Head and Clone repo update hash if needed
	cloneResult, err := cloneAndSetHash(compID)
	if err != nil {
		log.Error.Println("Error checking out head and cloning", err)
		return "", err
	}
	defer os.RemoveAll(cloneResult.path)
	tarNameExt, err := buildComponentBundle(tarName, cloneResult.path)
	if err != nil {
		log.Error.Println("Error building component bundle", err)
		return "", err
	}

	defer os.Remove("./" + tarNameExt)

	// Send .tar to worker
	log.Info.Println("SCP tar to worker...")
	source := "./" + tarNameExt
	destination := "/home/ubuntu/" + tarNameExt
	err = scpToWorker(worker.URL, source, destination, tarNameExt)
	if err != nil {
		log.Error.Println("Error copying to worker", err)
		return "", err
	}

	// Ensure hash is consistent
	compID.Hash = cloneResult.hash

	// Activate Component
	err = worker.Activate(compID, destination)
	if err != nil {
		log.Error.Println("Error activating worker", err)
		return "", err
	}

	return cloneResult.hash, nil
}

func (a *Activator) Deactivate(compID worker.ComponentID, worker *worker.V9Worker) error {
	return worker.Deactivate(compID)
}

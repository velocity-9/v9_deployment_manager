package deployment

import (
	"fmt"
	"time"
	"v9_deployment_manager/database"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"
)

type AutoScaler struct {
	actionManager *ActionManager
	driver        *database.Driver
	workers       []*worker.V9Worker
}

type ComponentStatAndInstances struct {
	numInstances int
	averageStats worker.ComponentStats
}

var MaxHits = 100.0
var MinHits = 15.0

func (scaler *AutoScaler) AutoScale() {
	//Get formatted worker ids
	// TODO: Pull this out into a helper function
	workerIDs := make([]string, len(scaler.workers))
	for i := range scaler.workers {
		name := fmt.Sprintf("worker_%d", i)
		id, err := scaler.driver.FindWorkerID(name)
		if err != nil {
			log.Error.Println("error getting worker id:", err)
			continue
		}
		workerIDs[i] = id
	}

	// Collect status of each comp on each worker
	compMap := make(map[worker.ComponentID]*ComponentStatAndInstances)
	for _, w := range scaler.workers {
		status, err := w.Status()
		if err != nil {
			log.Warning.Println("error getting worker status:", err)
			continue
		}

		// Keep track of what components are running
		for _, componentStats := range status.ActiveComponents {
			cID := componentStats.ID
			if _, ok := compMap[cID]; ok {
				//If CID already in map then average Hits
				compMap[cID].numInstances++
				compMap[cID].averageStats.Hits += componentStats.Hits
				compMap[cID].averageStats.Hits /= float64(compMap[cID].numInstances)
			} else {
				compMap[cID] = &ComponentStatAndInstances{1, componentStats}
			}
		}
	}

	log.Info.Println("----------------------------")
	for _, stats := range compMap {
		hits := stats.averageStats.Hits
		repo := stats.averageStats.ID.Repo
		log.Info.Println("repo: ", repo, "hits: ", hits)
		//Evaluate if scaling up is needed
		if stats.averageStats.Hits > MaxHits {
			//FIXME Update num instances in db
			//scaler.actionManager.NotifyComponentStateChanged()
			log.Info.Println("NEED MOAR POWERRR")
		}
		//Evaluate if scaling down is needed
		if stats.numInstances > 1 && stats.averageStats.Hits < MinHits {
			//FIXME Update num instances in db
			//scaler.actionManager.NotifyComponentStateChanged()
			log.Info.Println("I shud prolly scale DOWN repo: ", repo, "instances: ", stats.numInstances)
		}
	}
}

func StartAutoScaler(actionManager *ActionManager, driver *database.Driver, workers []*worker.V9Worker, cadence time.Duration) {
	scaler := AutoScaler{
		actionManager: actionManager,
		driver:        driver,
		workers:       workers,
	}

	go func() {
		for {
			scaler.AutoScale()
			time.Sleep(cadence)
		}
	}()
}

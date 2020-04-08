package database

import (
	"fmt"
	"time"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"
)

type PollingPopulator struct {
	workers []*worker.V9Worker
	driver  *Driver
}

func (populator *PollingPopulator) pollWorkersToDatabase() {
	workerIDs := make([]string, len(populator.workers))
	for i := range populator.workers {
		// TODO: This name should come from the worker itself
		name := fmt.Sprintf("worker_%d", i)
		id, err := populator.driver.FindWorkerID(name)
		if err != nil {
			log.Error.Println("error getting worker id:", err)
			continue
		}
		workerIDs[i] = id
	}

	for i, w := range populator.workers {
		// TODO: Populate the CPU usage/memory usage/network usage
		status, err := w.Status()
		if err != nil {
			log.Warning.Println("error getting worker status:", err)
			continue
		}

		// TODO: Clear out old stats

		// Keep track of what components are running
		runningComps := make([]worker.ComponentID, 0)
		for _, componentStats := range status.ActiveComponents {
			runningComps = append(runningComps, componentStats.ID)

			err = populator.driver.InsertStats(workerIDs[i], componentStats)
			if err != nil {
				log.Warning.Println("error inserting stats in database:", err)
			}
			continue
		}

		// Update the running components table
		err = populator.driver.SetWorkerRunningComponents(workerIDs[i], runningComps)
		if err != nil {
			log.Warning.Println("error setting running components:", err)
		}
	}

	for i, w := range populator.workers {
		logs, err := w.Logs()
		if err != nil {
			log.Warning.Println("error getting worker logs:", err)
			continue
		}

		// TODO: Clear out old logs
		// TODO: Handle worker shutdown elegantly

		for _, componentLog := range logs.Logs {
			err = populator.driver.InsertLog(workerIDs[i], componentLog)
			if err != nil {
				log.Warning.Println("error inserting logs in database:", err)
			}
			continue
		}
	}
}

func StartPollingPopulator(workers []*worker.V9Worker, cadence time.Duration, driver *Driver) {
	populator := PollingPopulator{
		workers: workers,
		driver:  driver,
	}

	go func() {
		for {
			populator.pollWorkersToDatabase()
			time.Sleep(cadence)
		}
	}()
}

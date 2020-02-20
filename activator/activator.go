package activator

import (
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

func (a *Activator) Activate(compID *worker.ComponentID, worker *worker.V9Worker, tarLocation string) error {
	// Setup the DB deploying entry
	err := a.driver.EnterDeploymentEntry(compID)
	if err != nil {
		log.Error.Println("Error starting deploy using db:", err)
		return err
	}
	defer func() {
		purgeErr := a.driver.PurgeDeploymentEntry(compID)
		if purgeErr != nil {
			log.Error.Println("Error purging deployment entry:", purgeErr)
		}
	}()

	// Activate Component
	err = worker.Activate(*compID, tarLocation)
	if err != nil {
		log.Error.Println("Error activating worker", err)
		return err
	}
	return nil
}

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

func (a *Activator) Activate(compID *worker.ComponentID, worker *worker.V9Worker) error {
	err = worker.Activate(*compID, destination)
	if err != nil {
		log.Error.Println("Error activating worker", err)
		return err
	}
	return nil
}

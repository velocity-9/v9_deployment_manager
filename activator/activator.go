package activator

import (
	guuid "github.com/google/uuid"
	"os"

	"v9_deployment_manager/database"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"
)

type Activator struct {
	driver *database.Driver
}

func CreateActivator(driver *database.Driver) (*Activator, error) {
	return &Activator{
		driver: driver,
	}, nil
}

func (a *Activator) Activate(compID *worker.ComponentID, worker *worker.V9Worker) error {
	// Get random tar name
	tarName := guuid.New().String()
	fullRepoName := compID.User + "/" + compID.Repo
	// Get Repo Contents
	log.Info.Println("Cloning Repo...")
	clonedPath, err := cloneRepo(fullRepoName)
	if err != nil {
		log.Error.Println("Error cloning repo:", err)
		return err
	}
	defer os.RemoveAll(clonedPath) // clean up

	//FIXME git checkout HEAD/hash
	if compID.Hash == "HEAD" {
		compID.Hash, err = getHash(clonedPath)
		if err != nil {
			log.Error.Println("Error getting hash from repo:", err)
			return err
		}
	}
	// Setup the DB deploying entry
	err = a.driver.EnterDeploymentEntry(compID)
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

	// Build image
	log.Info.Println("Building image from Dockerfile...")
	err = buildImageFromDockerfile(tarName, clonedPath)
	if err != nil {
		log.Error.Println("Error building image from Dockerfile", err)
		return err
	}

	// Build and Zip Tar
	log.Info.Println("Building and zipping tar...")
	tarNameExt, err := buildAndZipTar(tarName)
	if err != nil {
		log.Error.Println("Failed to build and compress tar", err)
		return err
	}
	defer os.Remove("./" + tarNameExt)

	// Send .tar to worker
	log.Info.Println("SCP tar to worker...")
	source := "./" + tarNameExt
	destination := "/home/ubuntu/" + tarNameExt
	err = scpToWorker(worker.URL, source, destination, tarNameExt)
	if err != nil {
		log.Error.Println("Error copying to worker", err)
		return err
	}

	err = worker.Activate(*compID, destination)
	if err != nil {
		log.Error.Println("Error activating worker", err)
		return err
	}
	return nil
}

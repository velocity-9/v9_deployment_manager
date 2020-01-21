package main

import (
	"os"
	"sync"

	guuid "github.com/google/uuid"
)

type Deployer struct {
	allWorkers []*V9Worker

	deploymentChannelMutex sync.RWMutex
	deploymentChannels map[repoPath]chan deploymentInfo
}

type repoPath struct {
	repo string
	user string
}

type deploymentInfo struct {
	id componentID
	downloadURL string
	worker *V9Worker
}

func(deployer *Deployer) deploy(id componentID, downloadURL string, targetWorker *V9Worker) {
	path := repoPath {
		repo: id.Repo,
		user: id.User,
	}

	deployer.deploymentChannelMutex.RLock()

	pusher, ok := deployer.deploymentChannels[path]

	if !ok {
		deployer.deploymentChannelMutex.RUnlock()
		deployer.deploymentChannelMutex.Lock()

		pusher, ok = deployer.deploymentChannels[path]
		if !ok {
			pusher = make(chan deploymentInfo, 1)

			go func() {
				for {
					depInfo := <-pusher
					deployComponentFromGithub(depInfo, deployer.allWorkers)
				}
			}()

			deployer.deploymentChannels[path] = pusher
		}

		deployer.deploymentChannelMutex.Unlock()
		deployer.deploymentChannelMutex.RLock()
	}

	// We push for a deployment if there is space in the channel -- AKA no other deployment is running
	select {
	case pusher<-deploymentInfo{
		id: id,
		downloadURL: downloadURL,
		worker:      targetWorker,
	}:
	default:
		// Do nothing if there is something already waiting to trigger
	}
}


func deployComponentFromGithub(depInfo deploymentInfo, workers []*V9Worker) {
	// Get random tar name
	// This is done early to have a unique temporary directory
	tarName := guuid.New().String()

	// Get Repo Contents
	Info.Println("Downloading Repo...")
	tempRepoPath := "./git_" + tarName
	err := downloadRepo(depInfo.downloadURL, tempRepoPath)
	if err != nil {
		Error.Println("Error downloading repo:", err)
	}
	defer os.RemoveAll(tempRepoPath)

	// Build image
	Info.Println("Building image from Dockerfile...")
	err = buildImageFromDockerfile(tarName, tempRepoPath)
	if err != nil {
		Error.Println("Error building image from Dockerfile", err)
		return
	}

	// Build and Zip Tar
	Info.Println("Building and zipping tar...")
	tarNameExt, err := buildAndZipTar(tarName)
	if err != nil {
		Error.Println("Failed to build and compress tar", err)
		return
	}
	defer os.Remove("./" + tarNameExt)

	// Send .tar to worker
	Info.Println("SCP tar to worker...")
	source := "./" + tarNameExt
	destination := "/home/ubuntu/" + tarNameExt
	err = scpToWorker(depInfo.worker.url, source, destination, tarNameExt)
	if err != nil {
		Error.Println("Error copying to worker", err)
		return
	}

	// Call deactivate to remove running component
	DeactivateComponentEverywhere(depInfo.id, workers)

	err = depInfo.worker.Activate(depInfo.id, destination)
	if err != nil {
		Error.Println("Error activating worker", err)
		return
	}
}

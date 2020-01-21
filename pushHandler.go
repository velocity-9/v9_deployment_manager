package main

import (
	"net/http"
	"os"
	"sync"

	guuid "github.com/google/uuid"
	"github.com/hjaensch7/webhooks/github"
)

type pushHandler struct {
	workers []*V9Worker
	counter int

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


func (h *pushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse worker param
	worker := h.workers[h.counter]
	h.counter = (h.counter + 1) % len(h.workers)
	// Load secret from env
	secret, exists := os.LookupEnv("GITHUB_SECRET")
	if !exists {
		Error.Println("Failed to find Github secret")
		return
	}
	// Setup github webhook
	hook, githubErr := github.New(github.Options.Secret(secret))
	if githubErr != nil {
		Error.Println("github.New Error:", githubErr)
	}
	// Parse push event or installation event from webhook
	// Note: integration events from github are ignored
	payload, err := hook.Parse(r, github.PushEvent, github.InstallationEvent, github.InstallationRepositoriesEvent)
	if err != nil {
		Error.Println("github payload parse error:", err)
		return
	}
	// Declare repo info vars
	var downloadURL string
	var user string
	var repo string
	// Send to Installation Handler if needed
	switch payload := payload.(type) {
	case github.InstallationPayload:
		Info.Println("Received Github App Installation Event...")
		Info.Println("Starting first time deployment...")
		downloadURL = getHTTPDownloadURLInstallation(payload)
		user = payload.Installation.Account.Login
		repo = payload.Repositories[0].Name
	case github.InstallationRepositoriesPayload:
		Info.Println("Received Github InstallationRepositories Event...")
		Info.Println("Starting first time deployment...")
		downloadURL = getHTTPDownloadURLInstallationRepositories(payload)
		user = payload.Installation.Account.Login
		repo = payload.RepositoriesAdded[0].Name
	default:
		parsedPayload := payload.(github.PushPayload)
		downloadURL = getHTTPDownloadURLPush(parsedPayload)
		user = parsedPayload.Repository.Owner.Login
		repo = parsedPayload.Repository.Name
	}

	path := repoPath{
		repo: repo,
		user: user,
	}

	h.deploymentChannelMutex.RLock()

	pusher, ok := h.deploymentChannels[path]

	if !ok {
		h.deploymentChannelMutex.RUnlock()
		h.deploymentChannelMutex.Lock()

		pusher, ok = h.deploymentChannels[path]
		if !ok {
			pusher = make(chan deploymentInfo, 1)

			go func() {
				for {
					depInfo := <-pusher
					deployComponentFromGithub(depInfo, h.workers)
				}
			}()

			h.deploymentChannels[path] = pusher
		}

		h.deploymentChannelMutex.Unlock()
		h.deploymentChannelMutex.RLock()
	}

	// We push for a deployment if there is space in the channel -- AKA no other deployment is running
	select {
	case pusher<-deploymentInfo{
			id:          componentID{
				User: user,
				Repo: repo,
				Hash: "test_hash",
			},
			downloadURL: downloadURL,
			worker:      worker,
		}:
	default:
		// Do nothing if there is something already waiting to trigger
	}

	h.deploymentChannelMutex.RUnlock()
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

type deploymentHandler struct {
	user string
	repo string
}

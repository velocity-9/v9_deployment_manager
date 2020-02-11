package main

import (
	"net/http"
	"os"

	guuid "github.com/google/uuid"
	"github.com/hjaensch7/webhooks/github"
)

type pushHandler struct {
	workers []*V9Worker
	counter int
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
	var repoNodeID string
	// Send to Installation Handler if needed
	switch payload := payload.(type) {
	case github.InstallationPayload:
		Info.Println("Received Github App Installation Event...")
		Info.Println("Starting first time deployment...")
		downloadURL = getHTTPDownloadURLInstallation(payload)
		user = payload.Installation.Account.Login
		repo = payload.Repositories[0].Name
		repoNodeID = payload.Repositories[0].NodeID
	case github.InstallationRepositoriesPayload:
		Info.Println("Received Github InstallationRepositories Event...")
		Info.Println("Starting first time deployment...")
		downloadURL = getHTTPDownloadURLInstallationRepositories(payload)
		user = payload.Installation.Account.Login
		repo = payload.RepositoriesAdded[0].Name
		repoNodeID = payload.RepositoriesAdded[0].NodeID
	default:
		parsedPayload := payload.(github.PushPayload)
		downloadURL = getHTTPDownloadURLPush(parsedPayload)
		user = parsedPayload.Repository.Owner.Login
		repo = parsedPayload.Repository.Name
		repoNodeID = parsedPayload.Repository.NodeID
	}

	compID := worker.ComponentID{User: user, Repo: repo, Hash: repoNodeID}

	// Setup the DB deploying entry
	err = h.driver.EnterDeploymentEntry(&compID)
	if err != nil {
		log.Error.Println("Error starting deploy using db:", err)
		return
	}
	defer func() {
		purgeErr := h.driver.PurgeDeploymentEntry(&compID)
		if purgeErr != nil {
			log.Error.Println("Error purging deployment entry:", purgeErr)
		}
	}()

	// Get random tar name
	// This is done early to have a unique temporary directory
	tarName := guuid.New().String()

	// Get Repo Contents
	Info.Println("Downloading Repo...")
	tempRepoPath := "./git_" + tarName
	err = downloadRepo(downloadURL, tempRepoPath)
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
	err = scpToWorker(worker.url, source, destination, tarNameExt)
	if err != nil {
		Error.Println("Error copying to worker", err)
		return
	}

	// Call deactivate to remove running component
	DeactivateComponentEverywhere(compID, h.workers)

	err = worker.Activate(compID, destination)
	if err != nil {
		Error.Println("Error activating worker", err)
		return
	}
}

package main

import (
	"net/http"
	"os"
	"v9_deployment_manager/database"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"

	guuid "github.com/google/uuid"
	"github.com/hjaensch7/webhooks/github"
)

type pushHandler struct {
	workers []*worker.V9Worker
	counter int
	driver  *database.Driver
}

func (h *pushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse worker param
	targetWorker := h.workers[h.counter]
	h.counter = (h.counter + 1) % len(h.workers)
	// Load secret from env
	secret, exists := os.LookupEnv("GITHUB_SECRET")
	if !exists {
		log.Error.Println("Failed to find Github secret")
		return
	}
	// Setup github webhook
	hook, githubErr := github.New(github.Options.Secret(secret))
	if githubErr != nil {
		log.Error.Println("github.New Error:", githubErr)
	}
	// Parse push event or installation event from webhook
	// Note: integration events from github are ignored
	payload, err := hook.Parse(r, github.PushEvent, github.InstallationEvent, github.InstallationRepositoriesEvent)
	if err != nil {
		log.Error.Println("github payload parse error:", err)
		return
	}
	// Declare repo info vars
	var repoFullName string
	var user string
	var repo string
	var hash = ""
	// Send to Installation Handler if needed
	switch payload := payload.(type) {
	case github.InstallationPayload:
		log.Info.Println("Received Github App Installation Event...")
		repoFullName = payload.Repositories[0].FullName
		user = payload.Installation.Account.Login
		repo = payload.Repositories[0].Name
	case github.InstallationRepositoriesPayload:
		log.Info.Println("Received Github InstallationRepositories Event...")
		repoFullName = payload.RepositoriesAdded[0].FullName
		user = payload.Installation.Account.Login
		repo = payload.RepositoriesAdded[0].Name
	default:
		parsedPayload := payload.(github.PushPayload)
		repoFullName = parsedPayload.Repository.FullName
		user = parsedPayload.Repository.Owner.Login
		repo = parsedPayload.Repository.Name
		hash = parsedPayload.HeadCommit.ID //Head commit hash
	}

	// Get random tar/repo name
	tarName := guuid.New().String()

	// Get Repo Contents
	log.Info.Println("Cloning Repo...")
	clonedPath, err := cloneRepo(repoFullName)
	if err != nil {
		log.Error.Println("Error cloning repo:", err)
		return
	}
	defer os.RemoveAll(clonedPath) // clean up

	//If first time installation get hash
	if hash == "" {
		hash, err = getHash(clonedPath)
		if err != nil {
			log.Error.Println("Error getting hash from repo:", err)
			return
		}
	}

	log.Info.Println("Building Component ID")
	log.Info.Println("User:", user)
	log.Info.Println("Repo:", repo)
	log.Info.Println("Hash:", hash)
	compID := worker.ComponentID{User: user, Repo: repo, Hash: hash}

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

	// Build image
	log.Info.Println("Building image from Dockerfile...")
	err = buildImageFromDockerfile(tarName, clonedPath)
	if err != nil {
		log.Error.Println("Error building image from Dockerfile", err)
		return
	}

	// Build and Zip Tar
	log.Info.Println("Building and zipping tar...")
	tarNameExt, err := buildAndZipTar(tarName)
	if err != nil {
		log.Error.Println("Failed to build and compress tar", err)
		return
	}
	defer os.Remove("./" + tarNameExt)

	// Send .tar to worker
	log.Info.Println("SCP tar to worker...")
	source := "./" + tarNameExt
	destination := "/home/ubuntu/" + tarNameExt
	err = scpToWorker(targetWorker.URL, source, destination, tarNameExt)
	if err != nil {
		log.Error.Println("Error copying to worker", err)
		return
	}

	// Call deactivate to remove running component
	worker.DeactivateComponentEverywhere(compID, h.workers)

	err = targetWorker.Activate(compID, destination)
	if err != nil {
		log.Error.Println("Error activating worker", err)
		return
	}
}

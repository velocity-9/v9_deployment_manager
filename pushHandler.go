package main

import (
	"net/http"
	"os"

	guuid "github.com/google/uuid"

	"v9_deployment_manager/activator"
	//	"v9_deployment_manager/database"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"

	"github.com/hjaensch7/webhooks/github"
)

type pushHandler struct {
	workers []*worker.V9Worker
	counter int
	//	driver    *database.Driver
	activator *activator.Activator
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
	var user string
	var repo string
	var hash = "HEAD"
	// Send to Installation Handler if needed
	switch payload := payload.(type) {
	case github.InstallationPayload:
		log.Info.Println("Received Github App Installation Event...")
		user = payload.Installation.Account.Login
		repo = payload.Repositories[0].Name
	case github.InstallationRepositoriesPayload:
		log.Info.Println("Received Github InstallationRepositories Event...")
		user = payload.Installation.Account.Login
		repo = payload.RepositoriesAdded[0].Name
	default:
		parsedPayload := payload.(github.PushPayload)
		user = parsedPayload.Repository.Owner.Login
		repo = parsedPayload.Repository.Name
		hash = parsedPayload.HeadCommit.ID //Head commit hash
	}
	compID := worker.ComponentID{User: user, Repo: repo, Hash: hash}

	// Get random tar name
	tarName := guuid.New().String()
	fullRepoName := compID.User + "/" + compID.Repo
	// Get Repo Contents
	log.Info.Println("Cloning " + compID.Repo + "...")
	clonedPath, err := cloneRepo(fullRepoName)
	if err != nil {
		log.Error.Println("Error cloning repo:", err)
		return
	}
	defer os.RemoveAll(clonedPath) // clean up

	err = checkoutHead(clonedPath)
	if err != nil {
		log.Error.Println("Ding dong the witch is dead", err)
	}
	if compID.Hash == "HEAD" {
		compID.Hash, err = getHash(clonedPath)
		if err != nil {
			log.Error.Println("Error getting hash from repo:", err)
			return
		}
	}
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

	err = h.activator.Activate(&compID, targetWorker, destination)
	if err != nil {
		log.Error.Println("Error activating worker", err)
		return
	}

}

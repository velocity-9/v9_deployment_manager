package main

import (
	"net/http"
	"os"

	"github.com/hjaensch7/webhooks/github"
)

type pushHandler struct {
	workers []*V9Worker
	counter int
	deployer *Deployer
}


func (h *pushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse worker param
	targetWorker := h.workers[h.counter]
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

	id := componentID{
		Repo: repo,
		User: user,
		Hash: "test_hash",
	}

	h.deployer.deploy(id, downloadURL, targetWorker)
}

type deploymentHandler struct {
	user string
	repo string
}

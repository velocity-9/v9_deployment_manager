package handlers

import (
	"net/http"
	"os"
	"v9_deployment_manager/database"
	"v9_deployment_manager/deployment"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"

	"github.com/hjaensch7/webhooks/github"
)

type PushHandler struct {
	actionManager *deployment.ActionManager
	driver *database.Driver
}

func NewPushHandler(actionManager *deployment.ActionManager, driver *database.Driver) *PushHandler {
	handler := PushHandler{
		actionManager: actionManager,
		driver: driver,
	}

	return &handler
}

func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Ensure we only deploy from master

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
		for _, repo := range payload.Repositories {
			compID := worker.ComponentID{User: user, Repo: repo.Name, Hash: hash}
			h.processComponentEvent(compID)
		}
	case github.InstallationRepositoriesPayload:
		log.Info.Println("Received Github InstallationRepositories Event...")
		user = payload.Installation.Account.Login
		for _, repo := range payload.RepositoriesAdded {
			compID := worker.ComponentID{User: user, Repo: repo.Name, Hash: hash}
			h.processComponentEvent(compID)
		}
	default:
		parsedPayload := payload.(github.PushPayload)
		user = parsedPayload.Repository.Owner.Login
		repo = parsedPayload.Repository.Name
		hash = parsedPayload.HeadCommit.ID

		compID := worker.ComponentID{User: user, Repo: repo, Hash: hash}
		h.processComponentEvent(compID)
	}
}

func (h *PushHandler) processComponentEvent(compID worker.ComponentID) {
	// We want to ensure that we have the database stuff for this user built up
	cID, err := h.driver.FindComponentID(&compID)
	if err != nil {
		log.Info.Println("Error finding database component id", err)
	} else {
		log.Info.Println("Received an event about component", compID, "| id =", cID)
	}

	// Then tell the action manager about the event
	h.actionManager.UpdateComponentHash(compID)
}
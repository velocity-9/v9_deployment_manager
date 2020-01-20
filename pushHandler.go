package main

import (
	"net/http"
	"os"

	guuid "github.com/google/uuid"
	"github.com/hjaensch7/webhooks/github"
)

type pushHandler struct {
	workers []string
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
	// Note: IntegrationInstallation and IntegrationInstallationRepositoriesEvents are ignored becuase they cause duplicate deployments
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
	switch payload.(type) {
	case github.InstallationPayload:
		Info.Println("Received Github App Installation Event...")
		Info.Println("Starting first time deployment...")
		parsedPayload := payload.(github.InstallationPayload)
		downloadURL = getHTTPDownloadURLInstallation(parsedPayload)
		user = parsedPayload.Installation.Account.Login
		repo = parsedPayload.Repositories[0].Name
	case github.InstallationRepositoriesPayload:
		Info.Println("Received Github InstallationRepositories Event...")
		Info.Println("Starting first time deployment...")
		parsedPayload := payload.(github.InstallationRepositoriesPayload)
		downloadURL = getHTTPDownloadURLInstallationRepositories(parsedPayload)
		user = parsedPayload.Installation.Account.Login
		repo = parsedPayload.RepositoriesAdded[0].Name
	default:
		parsedPayload := payload.(github.PushPayload)
		downloadURL = getHTTPDownloadURLPush(parsedPayload)
		user = parsedPayload.Repository.Owner.Login
		repo = parsedPayload.Repository.Name
	}
	dev := devID{user, repo, "test_hash"}

	// Get random tar name
	// This is done early to have a unique temporary directory
	tarName := guuid.New().String()

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
	err = scpToWorker(worker, source, destination, tarNameExt)
	if err != nil {
		Error.Println("Error copying to worker", err)
		return
	}

	// Call deactivate to remove running component
	deactivateComponent(dev, h.workers)

	err = activateWorker(dev, worker, destination)
	if err != nil {
		Error.Println("Error activating worker", err)
		return
	}
}

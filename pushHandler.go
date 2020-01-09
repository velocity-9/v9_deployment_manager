package main

import (
	"net/http"
	"os"

	guuid "github.com/google/uuid"
	"gopkg.in/go-playground/webhooks.v5/github"
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
	// Parse push event from webhook
	payload, err := hook.Parse(r, github.PushEvent)
	if err != nil {
		Error.Println("github payload parse error:", err)
		return
	}
	push := payload.(github.PushPayload)
	downloadURL := getHTTPDownloadURL(push)

	// Get Repo Contents
	Info.Println("Downloading Repo...")
	err = downloadRepo(downloadURL, tempRepoPath)
	if err != nil {
		Error.Println("Error downloading repo:", err)
	}
	defer os.RemoveAll(tempRepoPath)

	// Get random tar name
	tarName := guuid.New().String()

	// Build image
	Info.Println("Building image from Dockerfile...")
	err = buildImageFromDockerfile(tarName)
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

	// Activate worker
	user := push.Repository.Owner.Login
	repo := push.Repository.Name
	dev := devID{user, repo, "test_hash"}

	// Call deactivate to remove running component
	deactivateComponent(dev, h.workers)

	err = activateWorker(dev, worker, destination)
	if err != nil {
		Error.Println("Error activating worker", err)
		return
	}
}

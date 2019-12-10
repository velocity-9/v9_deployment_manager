package main

import (
	"net/http"
	"os"

	guuid "github.com/google/uuid"
	"gopkg.in/go-playground/webhooks.v5/github"
)

type pushHandler struct {
	worker string
}

func (h *pushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse worker param
	worker := h.worker
	// Setup github webhook
	hook, githubErr := github.New(github.Options.Secret("thespeedeq"))
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
	err = downloadRepo(downloadURL, "./temp_repo")
	if err != nil {
		Error.Println("Error downloading repo:", err)
	}
	defer os.RemoveAll("./temp_repo")

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
	tarNameExt := buildAndZipTar(tarName)
	if tarNameExt == "" {
		Error.Println("Failed to build and compress tar")
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
	err = activateWorker(dev, worker, destination)
	if err != nil {
		Error.Println("Error activating worker", err)
		return
	}
}

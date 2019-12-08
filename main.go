package main

import (
	guuid "github.com/google/uuid"
	"github.com/hashicorp/go-getter"
	"github.com/joho/godotenv"
	"gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
	"os"
)

func init() {
	// Setup log streams
	setLogStreams(os.Stdout, os.Stdout, os.Stderr)

	//Load .env file
	if err := godotenv.Load(); err != nil {
		Error.Print("No .env file found")
	}
}

func main() {
	// Get Environment variables
	path, port, worker := getEnvVariables()
	if path == "" {
		Error.Println("Error loading env variables")
	}

	hook, githubErr := github.New(github.Options.Secret("thespeedeq"))
	if githubErr != nil {
		Error.Println("github.New Error:", githubErr)
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
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
		dev := devId{user, repo, "test_hash"}
		err = activateWorker(dev, worker, destination, tarNameExt)
		if err != nil {
			Error.Println("Error activating worker", err)
			return
		}
	})

	Info.Println("Starting Server...")
	err := http.ListenAndServe(port, nil)
	if err != nil {
		Error.Println("http.http.ListenAndServe Error", err)
	}
}

// Get env variables
func getEnvVariables() (string, string, string) {
	path, exists := os.LookupEnv("ENDPOINT")
	if !exists {
		Error.Println("Failed to find ENDPOINT")
		return "", "", ""
	}
	port, exists := os.LookupEnv("PORT")
	if !exists {
		Error.Println("Failed to find PORT")
		return "", "", ""
	}
	worker, exists := os.LookupEnv("WORKER")
	if !exists {
		Error.Println("Failed to find Worker URL")
		return "", "", ""
	}
	return path, port, worker
}

// Build download url
func getHTTPDownloadURL(p github.PushPayload) string {
	return "git::" + p.Repository.URL
}

// Download repo contents to a specific location
func downloadRepo(downloadURL string, downloadLocation string) error {
	err := getter.Get(downloadLocation, downloadURL)
	if err != nil {
		return err
	}
	return nil
}

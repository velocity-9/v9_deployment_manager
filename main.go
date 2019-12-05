package main

import (
	guuid "github.com/google/uuid"
	"github.com/hashicorp/go-getter"
	"gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
	"os"
	"os/exec"
)

const (
	path = "/payload"
	port = "0.0.0.0:81"
)

func main() {
	// Setup log streams
	setLogStreams(os.Stdout, os.Stdout, os.Stderr)

	hook, err := github.New(github.Options.Secret("thespeedeq"))
	if err != nil {
		Error.Println("github.New Error:", err)
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			Error.Println("github payload parse error:", err)
			return
		}
		Info.Println("Received Payload:")
		push := payload.(github.PushPayload)
		downloadURL := getHTTPDownloadURL(push)
		Info.Println("Download URL:", downloadURL)

		// Get Repo Contents
		err = downloadRepo(downloadURL, "./temp_repo")
		if err != nil {
			Error.Println("Error downloading repo:", err)
		}

		// Get random tar name
		tar_name := guuid.New()

		// Build image
		Info.Println("Building image from Dockerfile...")
		err = buildImageFromDockerfile(tar_name.String())
		if err != nil {
			Error.Println("Error building image from Dockerfile", err)
		}

		// Build tar
		Info.Println("Building tar from Docker image...")
		err = buildTarFromImage(tar_name.String())
		if err != nil {
			Error.Println("Error building tar from image", err)
		}

		tarNameExt := tar_name.String() + ".tar"
		// Gzip tar
		Info.Println("Gzipping tar...")
		err = gzipTar(tarNameExt)
		if err != nil {
			Error.Println("Failure to gzip")
			return
		}
		tarNameExt = tarNameExt + ".gz"

		// Send .tar to worker
		source := "/home/ubuntu/go/src/v9_deployment_manager/" + tarNameExt
		destination := "/home/ubuntu/" + tarNameExt
		err = scpToWorker(source, destination, tarNameExt)
		if err != nil {
			Error.Println("Error copying to worker", err)
			return
		}

		// Activate worker
		user := push.Repository.Owner.Login
		repo := push.Repository.Name
		dev := dev_id{user, repo, "test_hash"}
		err = activateWorker(dev, "test", destination, tarNameExt)
		if err != nil {
			Error.Println("Error activating worker", err)
			return
		}

		// Cleanup
		err = os.RemoveAll("./temp_repo")
		if err != nil {
			Error.Println("Failed to remove temp_repo")
		}
		err = os.Remove("./" + tarNameExt)
		if err != nil {
			Error.Println("Failed to remove tar")
		}
	})

	Info.Println("Starting Server...")
	err = http.ListenAndServe(port, nil)
	if err != nil {
		Error.Println("http.http.ListenAndServe Error", err)
	}
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

// Build Docker Image Based on Dockerfile
func buildImageFromDockerfile(tarName string) error {
	cmd := exec.Command("docker", "build", "-t", tarName, "./temp_repo")
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// Build .tar from Docker Image
func buildTarFromImage(tarName string) error {
	tarNameExt := tarName + ".tar"
	cmd := exec.Command("docker", "save", tarName, "-o", tarNameExt)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// GZip tar
func gzipTar(tarName string) error {
	cmd := exec.Command("gzip", tarName)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

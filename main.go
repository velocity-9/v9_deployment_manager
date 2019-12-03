package main

import (
	guuid "github.com/google/uuid"
	"github.com/hashicorp/go-getter"
	"gopkg.in/go-playground/webhooks.v5/github"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

const (
	path = "/payload"
	port = ":3069"
)

var (
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func setLogStreams(
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {
	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(infoHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(infoHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

}

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

		// Get full repo name
		tar_name := guuid.New()

		// Build image
		err = buildImageFromDockerfile(tar_name.String())
		if err != nil {
			Error.Println("Error building image from Dockerfile", err)
		}

		// Build tar
		err = buildTarFromImage(tar_name.String())
		if err != nil {
			Error.Println("Error building tar from image", err)
		}
		Info.Println("Finished building tar from dockerfile")

		// Send .tar to worker

		// Activate worker
		tarNameExt := tar_name.String() + ".tar"
		err = activateWorker("test", tarNameExt)
		if err != nil {
			Error.Println("Error activating worker", err)
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

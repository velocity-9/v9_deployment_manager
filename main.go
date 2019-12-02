package main

import (
	"github.com/hashicorp/go-getter"
	"gopkg.in/go-playground/webhooks.v5/github"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
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
		tar_name := getFullRepoName(push)
		// Build image
		err = buildImageFromDockerfile(tar_name)
		if err != nil {
			Error.Println("Error building image from Dockerfile", err)
		}
		// Build tar
		err = buildTarFromImage(tar_name)
		if err != nil {
			Error.Println("Error building tar from image", err)
		}
	})

	Info.Println("Starting Server...")
	err = http.ListenAndServe(port, nil)
	if err != nil {
		Error.Println("http.http.ListenAndServe Error", err)
	}
}

func getHTTPDownloadURL(p github.PushPayload) string {
	return "git::" + p.Repository.URL
}

func getFullRepoName(p github.PushPayload) string {
	return strings.Replace(p.Repository.FullName, "/", "_", -1)
}

func downloadRepo(downloadURL string, downloadLocation string) error {
	err := getter.Get(downloadLocation, downloadURL)
	if err != nil {
		return err
	}
	return nil
}

func buildImageFromDockerfile(tarName string) error {
	// Build Docker Image Based on Dockerfile
	cmd := exec.Command("docker", "build", "-t", tarName, "./temp_repo")
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func buildTarFromImage(tarName string) error {
	tarNameExt := tarName + ".tar"
	// Build .tar from Docker Image
	cmd := exec.Command("docker", "save", tarName, "-o", tarNameExt)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

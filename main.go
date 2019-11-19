package main

import (
	"github.com/hashicorp/go-getter"
	"gopkg.in/go-playground/webhooks.v5/github"
	"io"
	"log"
	"net/http"
	"os"
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
	//Init Log streams
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
		err = downloadRepo(downloadURL, "./test_repo")
		if err != nil {
			Error.Println("Error downloading repo:", err)
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

func downloadRepo(downloadURL string, downloadLocation string) error {
	err := getter.Get(downloadLocation, downloadURL)
	if err != nil {
		return err
	}
	return nil
}

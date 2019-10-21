package main

import (
	"fmt"
	"github.com/hashicorp/go-getter"
	"gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
)

const (
	path = "/payload"
)

func main() {
	hook, err := github.New(github.Options.Secret("thespeedeq"))
	if err != nil {
		fmt.Println("github.New Error:", err)
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			fmt.Println("github payload parse error:", err)
			return
		}
		fmt.Println("Received Payload:")
		push := payload.(github.PushPayload)
		downloadURL := getHTTPDownloadURL(push)
		fmt.Println("Download URL: ", downloadURL)
		downloadRepo(downloadURL)
	})

	fmt.Println("Starting Server...")
	err = http.ListenAndServe(":3069", nil)
	if err != nil {
		fmt.Println("http.http.ListenAndServe Error", err)
	}
}

func getHTTPDownloadURL(p github.PushPayload) string {
	return "git::" + p.Repository.URL
}

func downloadRepo(downloadURL string) {
	err := getter.Get("./repo", downloadURL)
	if err != nil {
		fmt.Println("Error downloading repo:", err)
		return
	}
	fmt.Println("Repo downloaded from:", downloadURL)
}

package main

import (
	"fmt"
	"gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
)

const (
	path = "/payload"
)

func main() {
	hook, err := github.New(github.Options.Secret("thespeedeq"))
	if err != nil {
		fmt.Println("github.New Error", err)
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			fmt.Println("github payload parse failed", err)
			return
		}
		fmt.Println("Received Payload:")
		push := payload.(github.PushPayload)
		fmt.Println("Download URL: ", getHTTPDownloadURL(push))
	})

	fmt.Println("Starting Server...")
	err = http.ListenAndServe(":3069", nil)
	if err != nil {
		fmt.Println("http.http.ListenAndServe Error", err)
	}
}

func getHTTPDownloadURL(p github.PushPayload) string {
	return p.Repository.URL + ".git"
}

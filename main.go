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
	hook, err := github.New(github.Options.Secret("<GITHUBSECRET>"))
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
		fmt.Println("PUSH PAYLOAD:")
		fmt.Println(push)
	})

	fmt.Println("Starting Server...")
	http.ListenAndServe(":3069", nil)
}

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
	hook, _ := github.New(github.Options.Secret("thespeedeq!"))

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			if err == github.ErrEventNotFound {
				// ok event wasn't one of the ones asked to be parsed
				println("Error receiving github payload")
				fmt.Println(err)

			}
		}

		fmt.Printf("Received Payload:\n")
		push := payload.(github.PushPayload)
		fmt.Printf("PUSH PAYLOAD:\n %+v", push)

	})

	fmt.Printf("Starting Server...\n")
	http.ListenAndServe(":3069", nil)
}

package main

import (
	"errors"
	"net/http"
	"os"
	"strings"
)

func init() {
	// Setup log streams
	setLogStreams(os.Stdout, os.Stdout, os.Stderr)

}

func main() {
	//Initialize default ports
	CIPort := "0.0.0.0:81"
	websitePort := "0.0.0.0:80"

	// Check for development flag
	if len(os.Args) > 1 && contains(os.Args, "--development") {
		CIPort = ":3081"
		websitePort = ":3080"
	}

	// Get workers from env
	workers, envErr := getWorkers()
	if envErr != nil {
		Error.Println("Error loading env variables", envErr)
		return
	}

	Info.Println("CIPort", CIPort, "websitePort", websitePort, "workers", workers)

	go func() {
		Info.Println("Starting status handler...")
		http.Handle("/status", &statusHandler{workers: workers})
		err := http.ListenAndServe(websitePort, nil)
		if err != nil {
			Error.Println("Status http.ListenAndServer Error:", err)
		}
	}()

	http.Handle("/payload", &pushHandler{workers: workers, counter: 0})
	Info.Println("Starting Server...")
	err := http.ListenAndServe(CIPort, nil)
	if err != nil {
		Error.Println("CI http.ListenAndServe Error:", err)
	}
}

// Get env variables
func getWorkers() ([]string, error) {
	workerString, exists := os.LookupEnv("V9_WORKERS")
	if !exists {
		Error.Println("Failed to find Worker URLs")
		return nil, errors.New("failed to find WORKERS")
	}
	workerArr := strings.Split(workerString, ";")
	return workerArr, nil
}

//Contains FIXME this should be in the helper class
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

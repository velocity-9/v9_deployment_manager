package main

import (
	"errors"
	"net/http"
	"os"
)

const (
	CIPort      = "0.0.0.0:81"
	websitePort = "0.0.0.0:80"
)

func init() {
	// Setup log streams
	setLogStreams(os.Stdout, os.Stdout, os.Stderr)

}

func main() {
	// Get Environment variables
	worker, err := getEnvVariables()
	if err != nil {
		Error.Println("Error loading env variables")
		return
	}

	go func() {
		Info.Println("Starting status handler...")
		http.Handle("/status", &statusHandler{worker: worker})
		err := http.ListenAndServe(websitePort, nil)
		if err != nil {
			Error.Println("Status http.ListenAndServer Error:", err)
		}
	}()

	http.Handle("/payload", &pushHandler{worker: worker, counter: 0})
	Info.Println("Starting Server...")
	err = http.ListenAndServe(CIPort, nil)
	if err != nil {
		Error.Println("CI http.ListenAndServe Error:", err)
	}
}

// Get env variables
func getEnvVariables() ([]string, error) {
	workerArr := make([]string, 2, 5)
	worker, exists := os.LookupEnv("WORKER1")
	if !exists {
		Error.Println("Failed to find Worker URL")
		return nil, errors.New("Failed to find WORKER1")
	}
	workerArr[0] = worker
	worker, exists = os.LookupEnv("WORKER2")
	if !exists {
		Error.Println("Failed to find Worker URL")
		return nil, errors.New("Failed to find WORKER2")
	}
	workerArr[1] = worker
	return workerArr, nil
}

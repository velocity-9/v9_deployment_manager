package main

import (
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	// Setup log streams
	setLogStreams(os.Stdout, os.Stdout, os.Stderr)

	//Load .env file
	if err := godotenv.Load(); err != nil {
		Error.Print("No .env file found")
	}
}

func main() {
	// Get Environment variables
	CIPort, websitePort, worker := getEnvVariables()
	if CIPort == "" || websitePort == "" || worker == "" {
		Error.Println("Error loading env variables")
	}

	go func() {
		Info.Println("Starting status handler...")
		http.Handle("/status", &statusHandler{worker: worker})
		err := http.ListenAndServe(websitePort, nil)
		if err != nil {
			Error.Println("Status http.ListenAndServer Error:", err)
		}
	}()

	http.Handle("/payload", &pushHandler{worker: worker})
	Info.Println("Starting Server...")
	err := http.ListenAndServe(CIPort, nil)
	if err != nil {
		Error.Println("CI http.ListenAndServe Error:", err)
	}
}

// Get env variables
func getEnvVariables() (string, string, string) {
	CIPort, exists := os.LookupEnv("CI_PORT")
	if !exists {
		Error.Println("Failed to find CI_PORT")
		return "", "", ""
	}
	websitePort, exists := os.LookupEnv("WEBSITE_PORT")
	if !exists {
		Error.Println("Failed to find WEBSITE_PORT")
		return "", "", ""
	}
	worker, exists := os.LookupEnv("WORKER2")
	if !exists {
		Error.Println("Failed to find Worker URL")
		return "", "", ""
	}
	return CIPort, websitePort, worker
}

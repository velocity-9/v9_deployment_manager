package main

import (
	"errors"
	"fmt"
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
		Error.Println("Error getting worker info", envErr)
		return
	}

	// Get psql info from env
	psqlInfo, psqlInfoErr := getPsqlInfo()
	if psqlInfoErr != nil {
		Error.Println("Error getting psql info", psqlInfoErr)
		return
	}

	Info.Println("CIPort", CIPort, "websitePort", websitePort, "workers", workers)

	dbErr := SetupDatabasePopulator(psqlInfo, workers)
	if dbErr != nil {
		Error.Println("Error connecting to DB", dbErr)
		return
	}

	http.Handle("/payload", &pushHandler{workers: workers, counter: 0})
	Info.Println("Starting Server...")
	err := http.ListenAndServe(CIPort, nil)
	if err != nil {
		Error.Println("CI http.ListenAndServe Error:", err)
	}
}

// Get env variables
func getEnvVar(name string) (string, error) {
	val, exists := os.LookupEnv(name)
	if !exists {
		return "", errors.New("Missing env variable: " + name)
	}

	return val, nil
}

func getWorkers() ([]*V9Worker, error) {
	workerString, err := getEnvVar("V9_WORKERS")
	if err != nil {
		return nil, err
	}

	workerUrls := strings.Split(workerString, ";")
	var workers []*V9Worker

	for _, url := range workerUrls {
		workers = append(workers, &V9Worker{url:url})
	}
	return workers, nil
}

func getPsqlInfo() (string, error) {
	pgHost, err := getEnvVar("V9_PG_HOST")
	if err != nil {
		return "", err
	}

	pgPort, err := getEnvVar("V9_PG_PORT")
	if err != nil {
		return "", err
	}

	pgUser, err := getEnvVar("V9_PG_USER")
	if err != nil {
		return "", err
	}

	pgPassword, err := getEnvVar("V9_PG_PASSWORD")
	if err != nil {
		return "", err
	}

	pgDb, err := getEnvVar("V9_PG_DB")
	if err != nil {
		return "", err
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		pgHost, pgPort, pgUser, pgPassword, pgDb)

	return psqlInfo, nil
}

// FIXME: this should be in the helper class
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"v9_deployment_manager/activator"
	"v9_deployment_manager/database"
	"v9_deployment_manager/deployment"
	"v9_deployment_manager/handlers"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"

	_ "github.com/lib/pq"
)

const databasePollingInterval = time.Second * 3
const autoScalerPollingInterval = time.Second * 15

func main() {
	//Initialize default ports
	CIPort := "0.0.0.0:81"
	websitePort := "0.0.0.0:80"

	// Check for development flag
	if len(os.Args) > 1 && contains(os.Args, "--development") {
		CIPort = ":3081"
		websitePort = ":3080"
	}

	// Seed the random number generator
	rand.Seed(time.Now().Unix())

	// Get workers from env
	workers, envErr := getWorkers()
	if envErr != nil {
		log.Error.Println("Error getting worker info", envErr)
		return
	}

	// Get psql info from env
	psqlInfo, psqlInfoErr := getPsqlInfo()
	if psqlInfoErr != nil {
		log.Error.Println("Error getting psql info", psqlInfoErr)
		return
	}

	log.Info.Println("CIPort", CIPort, "websitePort", websitePort, "workers", workers)

	driver, dbErr := database.CreateDriver(psqlInfo)
	if dbErr != nil {
		log.Error.Println("Error connecting to DB", dbErr)
		return
	}

	// We don't want old deploying entries
	dbErr = driver.PurgeAllDeploymentEntries()
	if dbErr != nil {
		log.Error.Println("DB error", dbErr)
		return
	}

	database.StartPollingPopulator(workers, databasePollingInterval, driver)

	activator := activator.CreateActivator(driver)
	actionManager := deployment.NewActionManager(activator, driver, workers)
	// State may be dirty when we start
	actionManager.NotifyComponentStateChanged()

	//Start Auto Scaler
	deployment.StartAutoScaler(actionManager, driver, workers, autoScalerPollingInterval)

	http.Handle("/payload", handlers.NewPushHandler(actionManager, driver))
	http.Handle("/api/set_deployment_intention", handlers.NewDeploymentIntentionHandler(actionManager, driver))
	log.Info.Println("Starting Server...")
	err := http.ListenAndServe(CIPort, nil)
	if err != nil {
		log.Error.Println("CI http.ListenAndServe Error:", err)
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

func getWorkers() ([]*worker.V9Worker, error) {
	workerString, err := getEnvVar("V9_WORKERS")
	if err != nil {
		return nil, err
	}

	workerUrls := strings.Split(workerString, ";")
	var workers = make([]*worker.V9Worker, len(workerUrls))

	for i, url := range workerUrls {
		workers[i] = &worker.V9Worker{URL: url}
	}
	return workers, nil
}

func getPsqlInfo() (string, error) {
	pgHost, err := getEnvVar("V9_PG_HOST")
	if err != nil {
		return "", err
	}

	pgPortString, err := getEnvVar("V9_PG_PORT")
	if err != nil {
		return "", err
	}
	pgPort, err := strconv.Atoi(pgPortString)
	if err != nil {
		return "", fmt.Errorf("err: V9_PG_PORT must be a valid integer, was %s: %w", pgPortString, err)
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

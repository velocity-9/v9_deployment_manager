package main

import (
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"strconv"
	"time"
)

const (
	pollingInterval = 1 * time.Second
)

func SetupDatabasePopulator(psqlInfo string, workers []*V9Worker) error {
	db, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		return err
	}

	populator := databasePopulator{db: db, workers: workers}
	err = populator.recordWorkerIDs()
	if err != nil {
		return err
	}

	go func() {
		for {
			populator.pollWorkers2Database()
			// We only poll occasionally, so sleep in between polls
			time.Sleep(pollingInterval)
		}
	}()

	return nil
}

type databasePopulator struct {
	db                *sql.DB
	workerDatabaseIDs []string
	workers           []*V9Worker
}

func (populator *databasePopulator) recordWorkerIDs() error {
	var ids []string
	for i, _ := range populator.workers {
		name := "worker_" + strconv.Itoa(i+1)
		// Ensure that there is a record for the worker in the database
		updateQuery := "INSERT INTO v9.public.workers(worker_name) SELECT ($1) WHERE NOT exists(SELECT worker_id FROM v9.public.workers WHERE worker_name = $1)"
		_, err := populator.db.Exec(updateQuery, name)
		if err != nil {
			Warning.Println("Could setup workers table for "+name, ":", err)
			return err
		}

		// Then go get the name that's guaranteed to be there
		var id string
		err = populator.db.QueryRow("SELECT worker_id FROM v9.public.workers WHERE worker_name = $1", name).Scan(&id)
		if err != nil {
			Warning.Println("Could not get id for "+name, ":", err)
			return err
		}
		ids = append(ids, id)
		Info.Println("Id", i, "=", id)
	}
	populator.workerDatabaseIDs = ids
	return nil
}

func (populator *databasePopulator) getUserID(githubUsername string) (string, error) {
	var id string
	err := populator.db.QueryRow("SELECT user_id FROM v9.public.users WHERE github_username = $1", githubUsername).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (populator *databasePopulator) getComponentID(compID componentID) (string, error) {
	userID, err := populator.getUserID(compID.User)
	if err != nil {
		return "", err
	}

	// Ensure that there is a component in the database associated with this repo
	insertQuery := "INSERT INTO v9.public.components(user_id, github_repo, deployment_status) SELECT $1, $2, 'ready' WHERE NOT exists(SELECT component_id FROM v9.public.components WHERE user_id = $1 AND github_repo = $2)"
	_, err = populator.db.Exec(insertQuery, userID, compID.Repo)
	if err != nil {
		return "", err
	}

	// Get the ID (we know there must be one, since we ran the above insert query)
	var id string
	getQuery := "SELECT component_id FROM v9.public.components WHERE user_id = $1 AND github_repo = $2"
	err = populator.db.QueryRow(getQuery, userID, compID.Repo).Scan(&id)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (populator *databasePopulator) insertStatsForComponent(workerID string, componentStats ComponentStatus) error {
	compID, err := populator.getComponentID(componentStats.ID)
	if err != nil {
		Warning.Println("Error getting component ID:", err)
		return err
	}

	percentiles, err := json.Marshal(componentStats.LatencyPercentiles)
	if err != nil {
		Warning.Println("Error marshaling latency percentiles:", err)
		return err
	}

	insertQuery := "INSERT INTO v9.public.stats (worker_id, component_id, received_time, color, stat_window_seconds, hits, avg_response_bytes, avg_ms_latency, ms_latency_percentiles) VALUES ($1, $2, NOW(), $3, $4, $5, $6, $7, $8)"
	_, err = populator.db.Exec(insertQuery, workerID, compID, componentStats.Color, componentStats.StatWindow, componentStats.Hits, componentStats.AvgResponseBytes, componentStats.AvgMsLatency, string(percentiles))
	if err != nil {
		Warning.Println("Error pushing stats to database:", err)
		return err
	}

	return nil
}

func (populator *databasePopulator) insertLogsForComponent(workerID string, log ComponentLog) error {
	compID, err := populator.getComponentID(log.ID)
	if err != nil {
		Warning.Println("Error getting component ID:", err)
		return err
	}

	// Get data to write on top of
	var logID *string = nil
	var logText *string = nil
	var logError *string = nil
	getOriginalQuery := "SELECT log_id, log_text, log_error FROM v9.public.logs WHERE worker_id = $1 AND component_id = $2 AND execution_num = $3"
	err = populator.db.QueryRow(getOriginalQuery, workerID, compID, log.DedupNumber).Scan(&logID, &logText, &logError)
	if err != nil && err != sql.ErrNoRows  {
		Warning.Println("Error getting prev row data:", err)
		return err
	}

	if logID == nil {
		randomUUID, err := uuid.NewRandom()
		if err != nil {
			Warning.Println("Error generating random UUID:", err)
			return err
		}
		randomID := randomUUID.String()
		logID = &randomID
	}

	if log.Log != nil {
		logText = log.Log
	}

	if log.Error != nil {
		logError = log.Error
	}

	insertUpdateQuery := "INSERT INTO v9.public.logs (log_id, worker_id, component_id, execution_num, log_text, log_error, received_time) VALUES ($1, $2, $3, $4, $5, $6, NOW()) ON CONFLICT (log_id) DO UPDATE SET log_text = $5, log_error = $6, received_time = NOW()"
	_, err  = populator.db.Exec(insertUpdateQuery, logID, workerID, compID, log.DedupNumber, logText, logError)
	if err != nil {
		Warning.Println("Error doing final log database updated:", err)
		return err
	}
	return err
}

func (populator *databasePopulator) pollWorkers2Database() {
	for i, worker := range populator.workers {
		workerID := populator.workerDatabaseIDs[i]

		// TODO: Populate the CPU usage/memory usage/network usage
		status, err := worker.Status()
		if err != nil {
			Warning.Println("Error getting worker status:", err)
			continue
		}

		// TODO: Clear out old stats

		for _, componentStats := range status.ActiveComponents {
			// Swallow err here -- it's logged and reported in the insertStatsForComponent function
			_ = populator.insertStatsForComponent(workerID, componentStats)
		}
	}

	for i, worker := range populator.workers {
		workerID := populator.workerDatabaseIDs[i]

		logs, err := worker.Logs()
		if err != nil {
			Warning.Println("Error getting worker logs:", err)
			continue
		}

		// TODO: Clear out old logs (important in case of worker shutdown)

		for _, componentLog := range logs.Logs {
			// Swallow err here -- it's logged and reported in the insertLogsForComponent function
			_ = populator.insertLogsForComponent(workerID, componentLog)
		}
	}
}

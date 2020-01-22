package main

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	pollingInterval = 1 * time.Second
)

func SetupDatabasePopulator(psqlInfo string, workers []*V9Worker) (*databasePopulator, error) {
	db, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		return nil, err
	}

	populator := databasePopulator{db: db, workers: workers}
	err = populator.recordWorkerIDs()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			populator.pollWorkers2Database()
			// We only poll occasionally, so sleep in between polls
			time.Sleep(pollingInterval)
		}
	}()

	return &populator, nil
}

type databasePopulator struct {
	db                *sql.DB
	workerDatabaseIDs []string
	workers           []*V9Worker
}

func (populator *databasePopulator) startDeploying(id componentID) error {
	dbId, err := populator.getComponentID(id)
	if err != nil {
		return err
	}

	// Check if it's already being deployed somewhere, assuming entries older than 15 minutes are irrelevent
	checkQuery := `SELECT * FROM v9.public.deploying WHERE component_id = $1 AND age(received_time) < '15 minutes'`
	res, err := populator.db.Exec(checkQuery, dbId)

	// If we get no error, or anything other than sql.ErrNoRows, then we're in trouble -- bail out
	if err != sql.ErrNoRows {
		Error.Println("Deploying entry already exists", res, "err:", err)
		return err
	}

	// FIXME: Tiny race condition here, since checkQuery and updateQuery are seperated
	//        This is fine for demo but needs fixed in future
	updateQuery := `INSERT INTO v9.public.deploying(component_id) VALUES ($1)`
	_, err = populator.db.Exec(updateQuery, dbId)
	return err
}

func (populator *databasePopulator) stopDeploying(id componentID) {
	dbId, err := populator.getComponentID(id)
	if err != nil {
		Error.Println("Could not get dbID, err:", err)
		return
	}

	deleteQuery := `DELETE FROM v9.public.deploying WHERE component_id = $1`
	_, err = populator.db.Exec(deleteQuery, dbId)
	if err != nil {
		Error.Println("Could not delete component:", err)
	}
}

func (populator *databasePopulator) recordWorkerIDs() error {
	var ids = make([]string, len(populator.workers))
	for i := range populator.workers {
		name := "worker_" + strconv.Itoa(i+1)

		// Ensure that there is a record for the worker in the database
		updateQuery := `INSERT INTO v9.public.workers(worker_name)
 		SELECT ($1) WHERE NOT exists(SELECT worker_id FROM v9.public.workers WHERE worker_name = $1)`

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
		ids[i] = id
		Info.Println("Id", i, "=", id)
	}
	populator.workerDatabaseIDs = ids
	return nil
}

func (populator *databasePopulator) getUserID(githubUsername string) (string, error) {
	var id string
	err := populator.db.QueryRow(
		"SELECT user_id FROM v9.public.users WHERE github_username = $1", githubUsername).Scan(&id)
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
	insertQuery := `INSERT INTO v9.public.components(user_id, github_repo, deployment_intention)
 	SELECT $1, $2, 'active'
	WHERE NOT exists(SELECT component_id FROM v9.public.components WHERE user_id = $1 AND github_repo = $2)`

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

	insertQuery := `INSERT INTO v9.public.stats
    (worker_id, component_id, received_time, color,
    stat_window_seconds, hits, avg_response_bytes, avg_ms_latency, ms_latency_percentiles)
    VALUES ($1, $2, NOW(), $3, $4, $5, $6, $7, $8)`

	_, err = populator.db.Exec(
		insertQuery, workerID, compID, componentStats.Color, componentStats.StatWindow,
		componentStats.Hits, componentStats.AvgResponseBytes, componentStats.AvgMsLatency, string(percentiles))
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
	var logID *string
	var logText *string
	var logError *string

	getOriginalQuery := `SELECT log_id, log_text, log_error FROM v9.public.logs
	WHERE worker_id = $1 AND component_id = $2 AND execution_num = $3`

	err = populator.db.QueryRow(getOriginalQuery, workerID, compID, log.DedupNumber).Scan(&logID, &logText, &logError)
	if err != nil && err != sql.ErrNoRows {
		Warning.Println("Error getting prev row data:", err)
		return err
	}

	if logID == nil {
		randomUUID, uuidErr := uuid.NewRandom()
		if uuidErr != nil {
			Warning.Println("Error generating random UUID:", uuidErr)
			return uuidErr
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

	insertUpdateQuery := `INSERT INTO v9.public.logs
    (log_id, worker_id, component_id, execution_num, log_text, log_error, received_time)
    VALUES ($1, $2, $3, $4, $5, $6, NOW())
    ON CONFLICT (log_id) DO UPDATE SET log_text = $5, log_error = $6, received_time = NOW()`

	_, err = populator.db.Exec(insertUpdateQuery, logID, workerID, compID, log.DedupNumber, logText, logError)
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

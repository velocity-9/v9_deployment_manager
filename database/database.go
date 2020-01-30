package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"v9_deployment_manager/worker"

	"github.com/google/uuid"
)

type Driver struct {
	db *sql.DB
}

func CreateDriver(psqlInfo string) (*Driver, error) {
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}
	return &Driver{
		db: db,
	}, nil
}

// TODO: Consider using transactions throughout here

func (driver *Driver) FindUserID(githubUsername string) (string, error) {
	var userID string
	// NOTE: There is a bit of a hack here, where we set github_username = $1 (setting it to the same username)
	// This ensures that the user_id is actually returned no matter what
	upsertQuery := `INSERT INTO v9.public.users(email, github_username) VALUES (NULL, $1)
	ON CONFLICT (github_username) DO UPDATE SET github_username = $1 RETURNING user_id`
	err := driver.db.QueryRow(upsertQuery, githubUsername).Scan(&userID)

	if err != nil {
		return "", fmt.Errorf("could not find/create user id from database: %w", err)
	}

	return userID, nil
}

func (driver *Driver) FindComponentID(compID *worker.ComponentID) (string, error) {
	userID, err := driver.FindUserID(compID.User)
	if err != nil {
		return "", err
	}

	var compDBID string
	// NOTE: There is a bit of a hack here, where we set github_repo = $1 (setting it to the same repo)
	// This ensures that the component_id is actually returned no matter what
	upsertQuery := `INSERT INTO v9.public.components(user_id, github_repo, deployment_intention) VALUES ($1, $2, $3)
	ON CONFLICT (user_id, github_repo) DO UPDATE SET github_repo = $2 RETURNING component_id`
	err = driver.db.QueryRow(upsertQuery, userID, compID.Repo, "active").Scan(&compDBID)

	if err != nil {
		return "", fmt.Errorf("could not find/create component in database: %w", err)
	}

	return compDBID, nil
}

func (driver *Driver) FindWorkerID(workerName string) (string, error) {
	var workerID string
	// NOTE: There is a bit of a hack here, where we set worker_name = $1 (setting it to the same name)
	// This ensures that the worker_id is actually returned no matter what
	upsertQuery := `INSERT INTO v9.public.workers(worker_name) VALUES ($1)
	ON CONFLICT (worker_name) DO UPDATE SET worker_name = $1 RETURNING worker_id`

	err := driver.db.QueryRow(upsertQuery, workerName).Scan(&workerID)
	if err != nil {
		return "", fmt.Errorf("could not find/create worker in database: %w", err)
	}

	return workerID, nil
}

func (driver *Driver) InsertStats(workerID string, componentStatus worker.ComponentStats) error {
	compID, err := driver.FindComponentID(&componentStatus.ID)
	if err != nil {
		return fmt.Errorf("error getting component ID: %w", err)
	}

	percentiles, err := json.Marshal(componentStatus.LatencyPercentiles)
	if err != nil {
		return fmt.Errorf("error marshaling latency percentiles: %w", err)
	}

	insertQuery := `INSERT INTO v9.public.stats
    (worker_id, component_id, color, stat_window_seconds, hits,
     avg_response_bytes, avg_ms_latency, ms_latency_percentiles)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = driver.db.Exec(
		insertQuery, workerID, compID, componentStatus.Color, componentStatus.StatWindow,
		componentStatus.Hits, componentStatus.AvgResponseBytes, componentStatus.AvgMsLatency, string(percentiles))
	if err != nil {
		return fmt.Errorf("error sending stats to database: %w", err)
	}

	return nil
}

// TODO: Refactor to make cleaner
func (driver *Driver) InsertLog(workerID string, compLog worker.ComponentLog) error {
	compDBID, err := driver.FindComponentID(&compLog.ID)
	if err != nil {
		return fmt.Errorf("error getting comp id for logs: %w", err)
	}

	// Get data to write on top of
	var logID *string
	var logText *string
	var logError *string

	getOriginalQuery := `SELECT log_id, log_text, log_error FROM v9.public.logs
	WHERE worker_id = $1 AND component_id = $2 AND execution_num = $3`

	err = driver.db.QueryRow(getOriginalQuery, workerID, compDBID, compLog.DedupNumber).Scan(&logID, &logText, &logError)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting previous logs from database: %w", err)
	}

	if logID == nil {
		randomUUID, uuidErr := uuid.NewRandom()
		if uuidErr != nil {
			return fmt.Errorf("error creating a random UUID for the log table: %w", uuidErr)
		}
		randomID := randomUUID.String()
		logID = &randomID
	}

	if compLog.Log != nil {
		logText = compLog.Log
	}

	if compLog.Error != nil {
		logError = compLog.Error
	}

	upsertQuery := `INSERT INTO v9.public.logs
    (log_id, worker_id, component_id, execution_num, log_text, log_error, received_time)
    VALUES ($1, $2, $3, $4, $5, $6, NOW())
    ON CONFLICT (log_id) DO UPDATE SET log_text = $5, log_error = $6, received_time = NOW()`

	_, err = driver.db.Exec(upsertQuery, logID, workerID, compDBID, compLog.DedupNumber, logText, logError)
	if err != nil {
		return fmt.Errorf("error doing final log database update: %w", err)
	}
	return err

}

func (driver *Driver) EnterDeploymentEntry(compID *worker.ComponentID) error {
	compDBID, err := driver.FindComponentID(compID)
	if err != nil {
		return err
	}

	var deploymentStartTime string
	upsertQuery := `INSERT INTO v9.public.deploying(component_id, deployment_reason)
	VALUES ($1, $2) ON CONFLICT DO NOTHING
	RETURNING to_char(deployment_start_time::timestamp at time zone 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	err = driver.db.QueryRow(upsertQuery, compDBID, "initial_deployment").Scan(&deploymentStartTime)
	if err == nil {
		return fmt.Errorf("there was a previous deployment starting at: %s", deploymentStartTime)
	}

	if err != sql.ErrNoRows {
		return fmt.Errorf("error checking against previous deployments: %w", err)
	}

	return nil
}

func (driver *Driver) PurgeDeploymentEntry(compID *worker.ComponentID) error {
	compDBID, err := driver.FindComponentID(compID)
	if err != nil {
		return fmt.Errorf("could not get component ID to purge deploying entry: %w", err)
	}

	deleteQuery := `DELETE FROM v9.public.deploying WHERE component_id = $1`
	_, err = driver.db.Exec(deleteQuery, compDBID, "initial_deployment")
	if err != nil {
		return fmt.Errorf("could not delete from deploying table: %w", err)
	}

	return nil
}
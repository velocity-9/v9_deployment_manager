package main

import (
	"database/sql"
	_ "github.com/lib/pq"
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

	populator := databasePopulator{db: db, workers:workers}

	go func() {
		populator.pollWorkers2Database()
		// We only poll occasionally, so sleep in between polls
		time.Sleep(pollingInterval)
	}()

	return nil
}

type databasePopulator struct {
	db *sql.DB
	workers []*V9Worker
}

func (populator *databasePopulator) pollWorkers2Database() {
	Warning.Println("Pretending to poll...")
}
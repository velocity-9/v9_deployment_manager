package main

import (
	"encoding/json"
	//	"fmt"
	"io/ioutil"
	"net/http"
)

type statusHandler struct {
	worker string
}

type status struct {
	WorkerID string `json:"worker_id"`
	Status   string `json:"status"`
}

func (h *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Make call out to get worker 2 status
	Info.Println("Getting Worker Status...")
	workerURL := "http://" + h.worker + "/meta/status"
	resp, err := http.Get(workerURL)
	if err != nil {
		Error.Println("Failed to get status", err)
		return
	}
	defer resp.Body.Close()
	// Parse response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Error.Println("Failure to parse response from worker", err)
		return
	}
	Info.Println("Sending Status...")
	Info.Println(string(respBody))
	stat := &status{WorkerID: "2", Status: string(respBody)}
	workerStatus, err := json.Marshal(stat)
	if err != nil {
		Error.Println("Failed to convert status to JSON", err)
	}
	Info.Println("WorkerStatus", string(workerStatus))
	Info.Println("stat.workerID", stat.WorkerID)
	Info.Println("stat.status", stat.Status)
	json.NewEncoder(w).Encode(stat)

}

package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type statusHandler struct {
	workers []string
}

type status struct {
	WorkerID int    `json:"worker_id"`
	Status   string `json:"status"`
}

func (h *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Make call out to get worker 2 status
	Info.Println("Getting Worker Status...")
	var allStat []status = make([]status, len(h.workers))
	for index, worker := range h.workers {
		Info.Println("Collecting status from worker", index+1)
		workerURL := "http://" + worker + "/meta/status"
		resp, err := http.Get(workerURL)
		if err != nil {
			Error.Println("Failed to get status", err)
		}
		defer resp.Body.Close()
		// Read response
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			Error.Println("Failure to read response from worker", err)
		}
		workerStatus := status{WorkerID: (index + 1), Status: string(respBody)}
		allStat[index] = workerStatus
	}
	Info.Println("Sending Status...")

	// FIXME: This CORS workaround cannot be in the final version
	w.Header().Set("Access-Control-Allow-Origin", "*")

	err := json.NewEncoder(w).Encode(allStat)
	if err != nil {
		Error.Println("Failed to encode status", err)
		return
	}

}

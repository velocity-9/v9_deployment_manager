package main

import (
	"encoding/json"
	//	"fmt"
	"io/ioutil"
	"net/http"
)

type statusHandler struct {
	worker []string
}

type status struct {
	WorkerID string `json:"worker_id"`
	Status   string `json:"status"`
}

type allStatus []status

func (h *statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Make call out to get worker 2 status
	Info.Println("Getting Worker Status...")
	var allStat = allStatus{}
	for index, worker := range h.worker {
		Info.Println("Collecting status from worker", index+1)
		workerURL := "http://" + worker + "/meta/status"
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
		workerStatus := status{WorkerID: string(index + 1), Status: string(respBody)}
		allStat = append(allStat, workerStatus)
	}
	Info.Println("Sending Status...")
	err := json.NewEncoder(w).Encode(allStat)
	if err != nil {
		Error.Println("Failed to encode status", err)
		return
	}

}

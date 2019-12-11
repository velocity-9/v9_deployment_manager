package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type deactivateRequest struct {
	ID devID `json:"id"`
}

// Build activate post body
func createDeactivateBody(dev devID) ([]byte, error) {
	body, err := json.Marshal(deactivateRequest{dev})
	return body, err
}

// Activate worker
func deactivateWorker(dev devID, workerURL string) error {
	// Marshal information into json body
	body, err := createDeactivateBody(dev)
	if err != nil {
		Error.Println("Failed to create activation body", err)
		return err
	}

	// Make deactivate post request
	workerURL = "http://" + workerURL + "/meta/deactivate"
	resp, err := http.Post(workerURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		Error.Println("Failed to post", err)
		return err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Error.Println("Failure to parse response from worker", err)
		return err
	}

	Info.Println("Response from worker:", string(respBody))
	return nil
}

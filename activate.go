package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type devId struct {
	User string `json:"user"`
	Repo string `json:"repo"`
	Hash string `json:"hash"`
}

type activateRequest struct {
	ID              devId  `json:"id"`
	ExecutableFile  string `json:"executable_file"`
	ExecutionMethod string `json:"execution_method"`
}

// Build activate post body
func createActivateBody(dev devId, tarPath string, executionMethod string) ([]byte, error) {
	body, err := json.Marshal(activateRequest{dev, tarPath, executionMethod})
	return body, err
}

// Activate worker
func activateWorker(dev devId, workerUrl string, tarPath string, tarName string) error {
	// Marshal information into json body
	body, err := createActivateBody(dev, tarPath, "docker-archive")
	if err != nil {
		Error.Println("Failed to create activation body", err)
		return err
	}
	// Make activate post request
	resp, err := http.Post(workerUrl, "application/json", bytes.NewBuffer(body))
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
	return err
}

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

const (
	worker_url = "http://ec2-54-211-200-158.compute-1.amazonaws.com/meta/activate"
)

type dev_id struct {
	User string `json:"user"`
	Repo string `json:"repo"`
	Hash string `json:"hash"`
}

type activateRequest struct {
	Id               dev_id `json:"id"`
	Executable_file  string `json:"executable_file"`
	Execution_method string `json:"execution_method"`
}

// Build activate post body
func createActivateBody(dev dev_id, tarPath string, execution_method string) ([]byte, error) {
	body, err := json.Marshal(activateRequest{dev, tarPath, execution_method})
	return body, err
}

// Activate worker
func activateWorker(dev dev_id, worker string, tarPath string, tarName string) error {
	// Marshal information into json body
	body, err := createActivateBody(dev, tarPath, "docker-archive")
	if err != nil {
		Error.Println("Failed to create activation body", err)
		return err
	}
	// Make activate post request
	resp, err := http.Post(worker_url, "application/json", bytes.NewBuffer(body))
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

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type componentId struct {
	User string `json:"user"`
	Repo string `json:"repo"`
	Hash string `json:"hash"`
}

type activateRequest struct {
	ID              componentId `json:"id"`
	ExecutableFile  string      `json:"executable_file"`
	ExecutionMethod string      `json:"execution_method"`
}

func createActivateBody(dev componentId, tarPath string, executionMethod string) ([]byte, error) {
	body, err := json.Marshal(activateRequest{dev, tarPath, executionMethod})
	return body, err
}

type deactivateRequest struct {
	ID componentId `json:"id"`
}

// Build activate post body
func createDeactivateBody(dev componentId) ([]byte, error) {
	body, err := json.Marshal(deactivateRequest{dev})
	return body, err
}

type V9Worker struct {
	url string
}

func (worker *V9Worker) post(route string, body []byte) (*http.Response, error) {
	url := "http://" + worker.url + route
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		Error.Println("Failed to post", err)
		return nil, err
	}

	return resp, nil
}

func(worker *V9Worker) Activate(component componentId, tarPath string) error {
	// Marshal information into json body
	body, err := createActivateBody(component, tarPath, "docker-archive")
	if err != nil {
		Error.Println("Failed to create activation body", err)
		return err
	}

	// Make activate post request
	resp, err := worker.post("/meta/activate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Error.Println("Failure to read response from worker", err)
		return err
	}

	// TODO: Look for activate errors and store them somewhere
	Info.Println("Response from worker:", string(respBody))
	return nil
}

func(worker *V9Worker) Deactivate(component componentId) error {
	// Marshal information into json body
	body, err := createDeactivateBody(component)
	if err != nil {
		Error.Println("Failed to create deactivation body", err)
		return err
	}

	// Make deactivate post request
	resp, err := worker.post("/meta/deactivate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Error.Println("Failure to read response from worker", err)
		return err
	}

	// TODO: Look for deactivate errors and store them somewhere
	Info.Println("Response from worker:", string(respBody))
	return nil
}

// Deactivate component
func DeactivateComponentEverywhere(dev componentId, workers []*V9Worker) {
	for i := range workers {
		err := workers[i].Deactivate(dev)
		if err != nil {
			Info.Println("Failed to deactivate worker:", i, err)
			// This can fail and should fall through
		}
	}
}


package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	worker_url = "http://localhost:8082/meta/activate"
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
func createActivateBody(user string, repo string, hash string, tarPath string, execution_method string) ([]byte, error) {
	id := dev_id{user, repo, hash}
	body, err := json.Marshal(activateRequest{id, tarPath, execution_method})
	return body, err
}

// Move .tar file to worker

// Activate worker
func activateWorker(worker string, tarName string) error {
	path := "/home/hank/Desktop/go/src/v9_deployment_manager/" + tarName
	body, err := createActivateBody("test", "docker_archive", "test", path, "docker-archive")
	if err != nil {
		fmt.Println("Failed to create activation body")
		return err
	}

	resp, err := http.Post(worker_url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Failed to post", err)
	}

	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failure to parse response from worker", err)
	}

	fmt.Println("Response from worker:", string(respBody))

	return err

}

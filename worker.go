package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type V9Worker struct {
	url string
}

type componentID struct {
	User string `json:"user"`
	Repo string `json:"repo"`
	Hash string `json:"hash"`
}

type activateRequest struct {
	ID              componentID `json:"id"`
	ExecutableFile  string      `json:"executable_file"`
	ExecutionMethod string      `json:"execution_method"`
}

func createActivateBody(compID componentID, tarPath string, executionMethod string) ([]byte, error) {
	body, err := json.Marshal(activateRequest{compID, tarPath, executionMethod})
	return body, err
}

type deactivateRequest struct {
	ID componentID `json:"id"`
}

// Build activate post body
func createDeactivateBody(compID componentID) ([]byte, error) {
	body, err := json.Marshal(deactivateRequest{compID})
	return body, err
}

type ComponentStatus struct {
	ID componentID `json:"id"`

	Color      string  `json:"color"`
	StatWindow float64 `json:"stat_window_seconds"`

	Hits float64 `json:"hits"`

	AvgResponseBytes   float64   `json:"avg_response_bytes"`
	AvgMsLatency       float64   `json:"avg_ms_latency"`
	LatencyPercentiles []float64 `json:"ms_latency_percentiles"`
}

type StatusResponse struct {
	CPUUsage         float64           `json:"cpu_usage"`
	MemoryUsage      float64           `json:"memory_usage"`
	NetworkUsage     float64           `json:"network_usage"`
	ActiveComponents []ComponentStatus `json:"active_components"`
}

type ComponentLog struct {
	ID          componentID `json:"id"`
	DedupNumber uint64     `json:"dedup_number"`
	Log         *string     `json:"log"`
	Error       *string     `json:"error"`
}

type LogResponse struct {
	Logs []ComponentLog `json:"logs"`
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

func (worker *V9Worker) Activate(component componentID, tarPath string) error {
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

func (worker *V9Worker) Deactivate(component componentID) error {
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
func DeactivateComponentEverywhere(compID componentID, workers []*V9Worker) {
	for i := range workers {
		err := workers[i].Deactivate(compID)
		if err != nil {
			Info.Println("Failed to deactivate worker:", i, err)
			// This can fail and should fall through
		}
	}
}

func (worker *V9Worker) Logs() (LogResponse, error) {
	url := "http://" + worker.url + "/meta/logs"
	resp, err := http.Get(url)
	if err != nil {
		Error.Println("Failed to get status", err)
		return LogResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Error.Println("Failure to read response from worker", err)
		return LogResponse{}, err
	}

	var logResponse LogResponse
	err = json.Unmarshal(respBody, &logResponse)
	if err != nil {
		return LogResponse{}, err
	}

	return logResponse, nil
}

func (worker *V9Worker) Status() (StatusResponse, error) {
	url := "http://" + worker.url + "/meta/status"
	resp, err := http.Get(url)
	if err != nil {
		Error.Println("Failed to get status", err)
		return StatusResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Error.Println("Failure to read response from worker", err)
		return StatusResponse{}, err
	}

	var statusResponse StatusResponse
	err = json.Unmarshal(respBody, &statusResponse)
	if err != nil {
		return StatusResponse{}, err
	}

	return statusResponse, nil
}

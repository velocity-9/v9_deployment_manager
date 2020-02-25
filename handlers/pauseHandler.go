package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"v9_deployment_manager/database"
	"v9_deployment_manager/deployment"
	"v9_deployment_manager/log"
	"v9_deployment_manager/worker"
)

type APIHandler struct {
	driver        *database.Driver
	actionManager *deployment.ActionManager
}

type APIBody struct {
	ID            worker.ComponentPath `json:"id"`
	UpdatedStatus string               `json:"updatedStatus"`
}

func NewAPIHandler(actionManager *deployment.ActionManager, driver *database.Driver) *APIHandler {
	return &APIHandler{
		actionManager: actionManager,
		driver:        driver,
	}
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse Body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error.Println("Error reading body", err)
	}
	log.Info.Println(string(body))
	var p APIBody
	err = json.Unmarshal(body, &p)
	if err != nil {
		log.Error.Println("Failed to unmarshal body", err)
		return
	}
	log.Info.Println(p.ID, p.UpdatedStatus)
	// Update Database
	err = h.driver.SetComponentStatus(p.ID, p.UpdatedStatus)
	if err != nil {
		log.Error.Println("Failed to update status on database", err)
		return
	}
	// Notify Action Manager
	h.actionManager.NotifyComponentStateChanged()
	// Send Response
	fmt.Fprintf(w, "10/4 Good Buddy")
}

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

type SetDeploymentIntentionHandler struct {
	actionManager *deployment.ActionManager
	driver        *database.Driver
}

type SetDeploymentIntentionBody struct {
	ID                     worker.ComponentPath `json:"id"`
	NewDeploymentIntention string               `json:"new_deployment_intention"`
}

func NewDeploymentIntentionHandler(
	actionManager *deployment.ActionManager,
	driver *database.Driver) *SetDeploymentIntentionHandler {
	return &SetDeploymentIntentionHandler{
		actionManager: actionManager,
		driver:        driver,
	}
}

func (h *SetDeploymentIntentionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse Body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error.Println("Error reading body", err)
	}
	log.Info.Println(string(body))
	var p SetDeploymentIntentionBody
	err = json.Unmarshal(body, &p)
	if err != nil {
		log.Error.Println("Failed to unmarshal body", err)
		return
	}
	log.Info.Println(p.ID, p.NewDeploymentIntention)
	// Update Database
	err = h.driver.SetDeploymentIntention(p.ID, p.NewDeploymentIntention)
	if err != nil {
		log.Error.Println("Failed to update status on database", err)
		return
	}
	// Notify Action Manager
	h.actionManager.NotifyComponentStateChanged()
	// Send Response

	_, err = fmt.Fprintf(w, "10/4 Good Buddy")
	if err != nil {
		log.Error.Println("Failed to write back that everything worked", err)
	}
}

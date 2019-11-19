package main

import (
	"encoding/json"
	"net/http"

	fluxapi "github.com/fluxcd/flux/pkg/api"
)

const DockerHub = "DockerHub"

func init() {
	Sources[DockerHub] = handleDockerhub
}

func handleDockerhub(s fluxapi.Server, _ []byte, w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Repository struct {
			RepoName string `json:"repo_name"`
		} `json:"repository"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Cannot decode webhook payload", http.StatusBadRequest)
		log(DockerHub, err.Error())
		return
	}
	doImageNotify(s, w, r, p.Repository.RepoName)
}

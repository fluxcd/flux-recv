package main

import (
	"encoding/json"
	"net/http"

	fluxapi "github.com/fluxcd/flux/pkg/api"
)

const Quay = "Quay"

func init() {
	Sources[Quay] = handleQuay
}

func handleQuay(s fluxapi.Server, _ []byte, w http.ResponseWriter, r *http.Request, _ Endpoint) {
	type payload struct {
		RepoName string `json:"docker_url"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Cannot decode webhook payload", http.StatusBadRequest)
		log(Quay, err.Error())
		return
	}
	doImageNotify(s, w, r, p.RepoName)
}

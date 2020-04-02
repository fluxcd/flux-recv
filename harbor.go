package main

import (
	"encoding/json"
	"net/http"

	fluxapi "github.com/fluxcd/flux/pkg/api"
)

const Harbor = "Harbor"

func init() {
	Sources[Harbor] = handleHarbor
}

func handleHarbor(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != string(key) {
		http.Error(w, "The Harbor token does not match", http.StatusUnauthorized)
		log(Harbor, "missing or incorrect Authorization header (!= shared secret)")
		return
	}

	type payload struct {
		Type string `json:"type"`
		EventData struct {
			Resources []struct {
				ResourceURL string `json:"resource_url"`
			} `json:"resources"`
		} `json:"event_data"`
	}

	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Cannot decode webhook payload", http.StatusBadRequest)
		log(Harbor, err.Error())
		return
	}

	if p.Type != "pushImage" {
		http.Error(w, "Unexpected event type", http.StatusBadRequest)
		log(Harbor, "unexpected event type:", p.Type)
		return
	}

	// Harbor may send a collection of resources, but all those
	// resources belong to the same image repository while we
	// only need to notify Flux once. For sake of simplicity we
	// pick the first one.
	res := p.EventData.Resources[0]
	doImageNotify(s, w, r, res.ResourceURL)
}

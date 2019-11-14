package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	fluxapi "github.com/fluxcd/flux/pkg/api"
	fluxapi_v9 "github.com/fluxcd/flux/pkg/api/v9"
	fluxhttp "github.com/fluxcd/flux/pkg/http"
	fluxclient "github.com/fluxcd/flux/pkg/http/client"
	"github.com/fluxcd/flux/pkg/image"
)

type HookHandler func(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request)

var Sources = map[string]HookHandler{
	"DockerHub": handleDockerhub,
}

func HandlerFromEndpoint(baseDir, apiUrl string, ep Endpoint) (string, http.Handler, error) {
	// 1. find the relevant Source (e.g., DockerHub)
	sourceHandler, ok := Sources[ep.Source]
	if !ok {
		return "", nil, fmt.Errorf("unknown source %q, check sources.go for possible values", ep.Source)
	}

	// 2. load the key so it can be used in the handler, and get the
	// fingerprint so it can be used to route to this handler
	// TODO...
	key := []byte{}
	fingerprint := "abc123"

	apiClient := fluxclient.New(http.DefaultClient, fluxhttp.NewAPIRouter(), apiUrl, fluxclient.Token(""))

	// 3. construct a handler from the above
	return fingerprint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceHandler(apiClient, key, w, r)
	}), nil
}

// --- specific handlers

func handleDockerhub(s fluxapi.Server, _ []byte, w http.ResponseWriter, r *http.Request) {
	type payload struct {
		Repository struct {
			RepoName string `json:"repo_name"`
		} `json:"repository"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		fluxhttp.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	handleImageNotify(s, w, r, p.Repository.RepoName)
}

func handleImageNotify(s fluxapi.Server, w http.ResponseWriter, r *http.Request, img string) {
	ref, err := image.ParseRef(img)
	if err != nil {
		fluxhttp.WriteError(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	change := fluxapi_v9.Change{
		Kind: fluxapi_v9.ImageChange,
		Source: fluxapi_v9.ImageUpdate{
			Name: ref.Name,
		},
	}
	ctx := r.Context()
	// Ignore the error returned here as the sender doesn't care. We'll log any
	// errors at the daemon level.
	s.NotifyChange(ctx, change)

	w.WriteHeader(http.StatusOK)
}

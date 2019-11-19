package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	fluxapi "github.com/fluxcd/flux/pkg/api"
	fluxapi_v9 "github.com/fluxcd/flux/pkg/api/v9"
)

const GitLab = "GitLab"

func init() {
	Sources[GitLab] = handleGitlabPush
}

func handleGitlabPush(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Gitlab-Token") != string(key) {
		http.Error(w, "The Gitlab token does not match", http.StatusUnauthorized)
		log(GitLab, "missing or incorrect X-Gitlab-Token header (!= shared secret)")
		return
	}
	if event := r.Header.Get("X-Gitlab-Event"); event != "Push Hook" {
		http.Error(w, "Unexpected or missing X-Gitlab-Event", http.StatusBadRequest)
		log(GitLab, "unknown gitlab event header:", event)
		return
	}

	type gitlabPayload struct {
		Ref     string
		Project struct {
			SSHURL string `json:"git_ssh_url"`
		}
	}

	var payload gitlabPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Unable to parse hook payload", http.StatusBadRequest)
		log(GitLab, "unable to parse payload:", err.Error())
		return
	}

	change := fluxapi_v9.Change{
		Kind: fluxapi_v9.GitChange,
		Source: fluxapi_v9.GitUpdate{
			URL:    payload.Project.SSHURL,
			Branch: strings.TrimPrefix(payload.Ref, "refs/heads/"),
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	if err := s.NotifyChange(ctx, change); err != nil {
		http.Error(w, "Error forwarding hook", http.StatusInternalServerError)
		log(GitLab, "error from downstream:", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

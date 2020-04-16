package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v28/github"

	fluxapi "github.com/fluxcd/flux/pkg/api"
	fluxapi_v9 "github.com/fluxcd/flux/pkg/api/v9"
)

const GitHub = "GitHub"

func init() {
	Sources[GitHub] = handleGithubPush
}

func handleGithubPush(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request, _ Endpoint) {
	payload, err := github.ValidatePayload(r, key)
	if err != nil {
		http.Error(w, "The GitHub signature header is invalid.", 401)
		log(GitHub, "invalid signature:", err.Error())
		return
	}

	hook, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "Cannot parse payload", http.StatusBadRequest)
		log(GitHub, "could not parse payload:", err.Error())
		return
	}

	switch hook := hook.(type) {
	case *github.PingEvent:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Pong"))
	case *github.PushEvent:
		update := fluxapi_v9.GitUpdate{
			URL:    *hook.Repo.SSHURL,
			Branch: strings.TrimPrefix(*hook.Ref, "refs/heads/"),
		}
		change := fluxapi_v9.Change{
			Kind:   fluxapi_v9.GitChange,
			Source: update,
		}
		ctx := r.Context()
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		err := s.NotifyChange(ctx, change)
		if err != nil {
			select {
			case <-ctx.Done():
				http.Error(w, "Timed out waiting for response from downstream API", http.StatusRequestTimeout)
				log(GitHub, "timed out")
			default:
				http.Error(w, "Error while calling downstream API", http.StatusInternalServerError)
				log(GitHub, "error:", err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("unexpected hook kind, but OK"))
		log(GitHub, "unexpected webhook payload", fmt.Sprintf("received webhook: %T\n%s", hook, github.Stringify(hook)))
	}
}

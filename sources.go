package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v28/github"

	fluxapi "github.com/fluxcd/flux/pkg/api"
	fluxapi_v9 "github.com/fluxcd/flux/pkg/api/v9"
	fluxhttp "github.com/fluxcd/flux/pkg/http"
	fluxclient "github.com/fluxcd/flux/pkg/http/client"
	"github.com/fluxcd/flux/pkg/image"
)

const timeout = 10 * time.Second

type HookHandler func(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request)

var Sources = map[string]HookHandler{
	"DockerHub": handleDockerhub,
	"GitHub":    handleGithubPush,
}

func HandlerFromEndpoint(baseDir, apiUrl string, ep Endpoint) (string, http.Handler, error) {
	// 1. find the relevant Source (e.g., DockerHub)
	sourceHandler, ok := Sources[ep.Source]
	if !ok {
		return "", nil, fmt.Errorf("unknown source %q, check sources.go for possible values", ep.Source)
	}

	// 2. load the key so it can be used in the handler, and get the
	// digest so it can be used to route to this handler
	// TODO...
	key, err := ioutil.ReadFile(filepath.Join(baseDir, ep.KeyPath))
	if err != nil {
		return "", nil, fmt.Errorf("cannot load key from %q: %s", ep.KeyPath, err.Error())
	}

	sha := sha256.New()
	sha.Write(key)
	digest := fmt.Sprintf("%x", sha.Sum(nil))

	apiClient := fluxclient.New(http.DefaultClient, fluxhttp.NewAPIRouter(), apiUrl, fluxclient.Token(""))

	// 3. construct a handler from the above
	return digest, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceHandler(apiClient, key, w, r)
	}), nil
}

func log(msg ...interface{}) {
	fmt.Fprintln(os.Stderr, msg...)
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
		http.Error(w, "Cannot decode webhook payload", http.StatusBadRequest)
		log("DockerHub", err.Error())
		return
	}
	handleImageNotify(s, w, r, p.Repository.RepoName)
}

func handleImageNotify(s fluxapi.Server, w http.ResponseWriter, r *http.Request, img string) {
	ref, err := image.ParseRef(img)
	if err != nil {
		http.Error(w, "Cannot parse image in webhook payload", http.StatusBadRequest)
		log("could not parse image from hook payload:", img, ":", err.Error())
		return
	}
	change := fluxapi_v9.Change{
		Kind: fluxapi_v9.ImageChange,
		Source: fluxapi_v9.ImageUpdate{
			Name: ref.Name,
		},
	}
	ctx := r.Context()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	s.NotifyChange(ctx, change)
	w.WriteHeader(http.StatusOK)
}

func handleGithubPush(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, key)
	if err != nil {
		http.Error(w, "The GitHub signature header is invalid.", 401)
		log("GitHub", "invalid signature:", err.Error())
		return
	}

	hook, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, "Cannot parse payload", http.StatusBadRequest)
		log("GitHub", "could not parse payload:", err.Error())
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
				log("GitHub", "timed out")
			default:
				http.Error(w, "Error while calling downstream API", http.StatusInternalServerError)
				log("GitHub", "error:", err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("unexpected hook kind, but OK"))
		log("GitHub", "unexpected webhook payload", fmt.Sprintf("received webhook: %T\n%s", hook, github.Stringify(hook)))
	}
}

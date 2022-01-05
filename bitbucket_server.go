package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	fluxapi "github.com/fluxcd/flux/pkg/api"
	fluxapi_v9 "github.com/fluxcd/flux/pkg/api/v9"
	"github.com/google/go-github/v41/github"
	"golang.org/x/sync/errgroup"
)

const BitbucketServer = "BitbucketServer"

func init() {
	Sources[BitbucketServer] = handleBitbucketServerPush
}

func handleBitbucketServerPush(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request, _ Endpoint) {
	// See incomplete docs: https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html

	body, err := github.ValidatePayload(r, key)
	if err != nil {
		http.Error(w, "The signature header is invalid.", http.StatusUnauthorized)
		log(BitbucketServer, "invalid signature:", err.Error())
		return
	}
	if eventKey := r.Header.Get("X-Event-Key"); eventKey != "repo:refs_changed" {
		http.Error(w, "Unexpected or missing header X-Event-Key", http.StatusBadRequest)
		log(BitbucketServer, "unexpected X-Event-Key header:", eventKey)
		return
	}
	var event bitbucketRefsChangedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Unable to JSON decode payload", http.StatusBadRequest)
		log(BitbucketServer, "unable to decode payload:", err.Error())
		return
	}
	repoURL, ok := event.repoCloneLink("ssh")
	if !ok {
		http.Error(w, "Missing repository SSH clone link", http.StatusBadRequest)
		log(BitbucketServer, "missing repository SSH clone link")
		return
	}

	var grp errgroup.Group
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	for refID := range event.changeRefIDs("BRANCH") {
		branch := strings.TrimPrefix(refID, "refs/heads/")
		grp.Go(func() error {
			return s.NotifyChange(ctx, fluxapi_v9.Change{
				Kind: fluxapi_v9.GitChange,
				Source: fluxapi_v9.GitUpdate{
					URL:    repoURL,
					Branch: branch,
				},
			})
		})
	}
	if err := grp.Wait(); err != nil {
		http.Error(w, "Unable to process all push events", http.StatusInternalServerError)
		log(BitbucketServer, "error from downstream:", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

type bitbucketRefsChangedEvent struct {
	Repository struct {
		Links struct {
			Clone []struct {
				Href string
				Name string
			}
		}
	}
	Changes []struct {
		Ref struct {
			ID   string
			Type string
		}
	}
}

func (e *bitbucketRefsChangedEvent) repoCloneLink(name string) (string, bool) {
	for _, link := range e.Repository.Links.Clone {
		if link.Name == name {
			return link.Href, true
		}
	}
	return "", false
}

func (e *bitbucketRefsChangedEvent) changeRefIDs(typ string) map[string]bool {
	var refIDs map[string]bool
	for _, c := range e.Changes {
		if c.Ref.Type != typ {
			continue
		}
		if refIDs == nil {
			refIDs = make(map[string]bool)
		}
		refIDs[c.Ref.ID] = true
	}
	return refIDs
}

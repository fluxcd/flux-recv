package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	fluxapi "github.com/fluxcd/flux/pkg/api"
	fluxapi_v9 "github.com/fluxcd/flux/pkg/api/v9"
)

// Handily (not handily) Bitbucket's cloud and self-hosted products
// have different names for all the events and fields. This is for the
// events sent by the "Cloud" product (bitbucket.org):
// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
//
// (For completeness, the docs for the self-hosted Bitbucket "Server"
// are at
// https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html. The
// self-hosted version includes a signature in the header
// "X-Hub-Signature", but this is not present for "Cloud").

const BitbucketCloud = "BitbucketCloud"

func init() {
	Sources[BitbucketCloud] = handleBitbucketCloudPush
}

func handleBitbucketCloudPush(s fluxapi.Server, _ []byte, w http.ResponseWriter, r *http.Request) {
	if event := r.Header.Get("X-Event-Key"); event != "repo:push" {
		http.Error(w, "Unexpected or missing header X-Event-Key", http.StatusBadRequest)
		log(BitbucketCloud, "missing or incorrect X-Event-Key header:", event)
		return
	}

	type bitbucketCloudPayload struct {
		Repository bitbucketCloudRepository
		Push       struct {
			Changes []struct {
				New struct {
					Type, Name string
				}
			}
		}
	}

	var payload bitbucketCloudPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Unable to decode payload as JSON", http.StatusBadRequest)
		log(BitbucketCloud, "unable to decode payload:", err.Error())
		return
	}

	// The bitbucket.org events potentially contain many ref updates;
	// presumably, it bundles together e.g., the result of a `git
	// push` into one event. We only notify about one things at a time
	// though, so:
	//  - collect all the changes
	//  - send as many as we can before we reach our deadline.
	// That may mean we miss some, but this is best effort.
	//
	// NB a change can be to a branch or a tag; here we'll send both
	// through, since it's in principle possible to sync to a tag.

	repo := payload.Repository.RepoURL()
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	for i := range payload.Push.Changes {
		refChange := payload.Push.Changes[i].New
		change := fluxapi_v9.Change{
			Kind: fluxapi_v9.GitChange,
			Source: fluxapi_v9.GitUpdate{
				URL:    repo,
				Branch: refChange.Name,
			},
		}
		if err := s.NotifyChange(ctx, change); err != nil {
			http.Error(w, "Unable to process all push events", http.StatusInternalServerError)
			log(BitbucketCloud, "error from downstream:", err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// The fields of repository that we care about
type bitbucketCloudRepository struct {
	FullName string `json:"full_name"`
}

func (r bitbucketCloudRepository) RepoURL() string {
	return fmt.Sprintf("git@bitbucket.org:%s.git", r.FullName)
}

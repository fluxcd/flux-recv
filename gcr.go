package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	fluxapi "github.com/fluxcd/flux/pkg/api"
)

const GoogleContainerRegistry = "GoogleContainerRegistry"
const insert = "insert"
const tokenIndex = len("Bearer ")

type data struct {
	Action string `json:"action"`
	Digest string `json:"digest"`
	Tag    string `json:"tag"`
}

type payload struct {
	Message struct {
		Data         string    `json:"data"`
		MessageID    string    `json:"messageId"`
		PublishTime  time.Time `json:"publishTime"`
		Subscription string    `json:"subscription"`
	} `json:"message"`
}

type auth struct {
	Aud string `json:"aud"`
}

func init() {
	Sources[GoogleContainerRegistry] = handleGoogleContainerRegistry
}

func handleGoogleContainerRegistry(s fluxapi.Server, _ []byte, w http.ResponseWriter, r *http.Request, config Endpoint) {
	// authenticate based on config
	if config.GCR != nil {
		if err := authenticateRequest(&http.Client{}, r.Header.Get("Authorization"), config.GCR.Audience); err != nil {
			http.Error(w, "Cannot authorize request", http.StatusOK)
			log(GoogleContainerRegistry, err.Error())
			return
		}
	}

	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Cannot decode payload", http.StatusOK)
		log(GoogleContainerRegistry, err.Error())
		return
	}

	raw, _ := base64.StdEncoding.DecodeString(p.Message.Data)

	var d data
	json.Unmarshal(raw, &d)

	if strings.ToLower(d.Action) != insert {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("action is not an insert, moving on"))
		return
	}

	log(GoogleContainerRegistry, fmt.Sprintf("Update: %s", d.Tag))

	doImageNotify(s, w, r, d.Tag)
}

func authenticateRequest(c *http.Client, bearer string, audience string) (err error) {
	if len(bearer) < tokenIndex {
		return fmt.Errorf("Authorization header is missing or malformed: %v", bearer)
	}

	token := bearer[tokenIndex:]
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", token)

	resp, err := c.Get(url)
	if err != nil {
		return fmt.Errorf("Cannot verify authenticity of payload: %w", err)
	}

	var p auth
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return fmt.Errorf("Cannot decode auth payload: %w", err)
	}

	// check if we are the intended audience
	if p.Aud != audience {
		return fmt.Errorf("Payload received but intended for a different audience: %v", p.Aud)
	}

	return nil
}

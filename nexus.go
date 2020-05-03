package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	fluxapi "github.com/fluxcd/flux/pkg/api"
)

const Nexus = "Nexus"

func init() {
	Sources[Nexus] = handleNexus
}

func handleNexus(s fluxapi.Server, key []byte, w http.ResponseWriter, r *http.Request, e Endpoint) {
	if webhookID := r.Header.Get("X-Nexus-Webhook-Id"); webhookID != "rm:repository:component" {
		http.Error(w, "Unsupported webhook ID", http.StatusBadRequest)
		log(Nexus, "unsupported webhook ID:", webhookID)
		return
	}

	signature := r.Header.Get("X-Nexus-Webhook-Signature")
	if len(signature) == 0 {
		http.Error(w, "Signature is missing from header", http.StatusUnauthorized)
		log(Nexus, "missing X-Nexus-Webhook-Signature header")
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read payload", http.StatusBadRequest)
		log(Nexus, fmt.Errorf("could not read payload: %v", err))
		return
	}

	if !verifyHmacSignature(key, signature, b) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		log(Nexus, "invalid X-Nexus-Webhook-Signature")
		return
	}

	type payload struct {
		Action string `json:"action"`
		Component struct {
			Format string `json:"format"`
			Name   string `json:"name"`
		} `json:"component"`
	}

	var p payload
	if err := json.Unmarshal(b, &p); err != nil {
		http.Error(w, "Cannot decode webhook payload", http.StatusBadRequest)
		log(Nexus, err.Error())
		return
	}

	if p.Component.Format != "docker" || p.Action != "CREATED" {
		http.Error(w, "Ignoring component format", http.StatusBadRequest)
		log(Nexus, "ignoring action:", p.Action, "for asset format:", p.Component.Format)
		return
	}

	// The request Nexus makes contains no information about the
	// hostname of the Docker registry.
	img := p.Component.Name
	if e.RegistryHost != "" {
		img = strings.TrimRight(e.RegistryHost, "/") + "/" + img
	}
	doImageNotify(s, w, r, img)
}

func verifyHmacSignature(key []byte, signature string, payload []byte) bool {
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

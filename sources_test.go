package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper to create a downstream flux API which will check the /notify payload is as expected
func newDownstream(t *testing.T, expectedPayload string) *httptest.Server {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v11/notify", r.URL.Path)
		defer r.Body.Close()
		bytes, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedPayload, string(bytes))
		fmt.Fprintln(w, `{"status": "OK"}`)
	}))
	return downstream
}

// helper to load e.g., a payload from fixtures
func loadFixture(t *testing.T, file string) []byte {
	bytes, err := ioutil.ReadFile("test/fixtures/" + file)
	assert.NoError(t, err)
	return bytes
}

const expectedDockerhub = `{"Kind":"image","Source":{"Name":{"Domain":"","Image":"svendowideit/testhook"}}}`

// Test that a hook arriving at a DockerHub endpoint calls the
// downstream with an image update. Docs:
// https://docs.docker.com/docker-hub/webhooks/
func TestDockerHubSource(t *testing.T) {
	downstream := newDownstream(t, expectedDockerhub)
	defer downstream.Close()

	endpoint := Endpoint{Source: DockerHub, KeyPath: "dockerhub_key"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(loadFixture(t, "dockerhub_payload")))
	assert.NoError(t, err)

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

const expectedGithub = `{"Kind":"git","Source":{"URL":"git@github.com:Codertocat/Hello-World.git","Branch":"refs/tags/simple-tag"}}`

// Docs:
// https://developer.github.com/v3/activity/events/types/#pushevent
// and the headers
// https://developer.github.com/v3/repos/hooks/#webhook-headers
func Test_GitHubSource(t *testing.T) {
	downstream := newDownstream(t, expectedGithub)
	defer downstream.Close()

	// NB key created with
	//     ruby -rsecurerandom -e 'puts SecureRandom.hex(20)' > test/fixtures/github_key
	// as suggested in the GitHub docs.
	endpoint := Endpoint{Source: GitHub, KeyPath: "github_key"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	payload := loadFixture(t, "github_payload")

	c := hookServer.Client()

	// First using application/json
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-GitHub-Event", "push")
	req.Header.Add("X-Hub-Signature", genGithubMAC(payload, loadFixture(t, "github_key"))) // <-- same as in the endpoint

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	// Now using form encoded
	form := url.Values{}
	form.Add("payload", string(payload))
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, strings.NewReader(form.Encode()))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Github-Event", "push")
	req.Header.Add("X-Hub-Signature", genGithubMAC([]byte(form.Encode()), loadFixture(t, "github_key"))) // <-- same as in the endpoint

	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

// genGithubMAC generates the GitHub HMAC signature for a message provided the secret key
func genGithubMAC(message, key []byte) string {
	mac := hmac.New(sha512.New, key)
	mac.Write(message)
	signature := mac.Sum(nil)

	hexSignature := make([]byte, hex.EncodedLen(len(signature)))
	hex.Encode(hexSignature, signature)
	return "sha512=" + string(hexSignature)
}

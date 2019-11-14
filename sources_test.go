package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
		fmt.Fprintln(w, "OK")
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
// downstream with an image update
func TestDockerHubSource(t *testing.T) {
	downstream := newDownstream(t, expectedDockerhub)
	defer downstream.Close()

	endpoint := Endpoint{Source: "DockerHub", KeyPath: "dockerhub_rsa"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(loadFixture(t, "dockerhub_payload")))
	assert.NoError(t, err)

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

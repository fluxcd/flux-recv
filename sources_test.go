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
func newDownstream(t *testing.T, expectedPayload string, called *bool) *httptest.Server {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v11/notify", r.URL.Path)
		defer r.Body.Close()
		bytes, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedPayload, string(bytes))
		fmt.Fprintln(w, `{"status": "OK"}`)
		*called = true
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
func Test_DockerHubSource(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedDockerhub, &called)
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
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)
}

const expectedQuay = `{"Kind":"image","Source":{"Name":{"Domain":"quay.io","Image":"hiddeco/foo"}}}`

func Test_Quay(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedQuay, &called)
	defer downstream.Close()

	endpoint := Endpoint{Source: Quay, KeyPath: "quay_key"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(loadFixture(t, "quay_payload")))
	assert.NoError(t, err)

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)
}

const expectedGithub = `{"Kind":"git","Source":{"URL":"git@github.com:Codertocat/Hello-World.git","Branch":"refs/tags/simple-tag"}}`

// Docs:
// https://developer.github.com/v3/activity/events/types/#pushevent
// and the headers
// https://developer.github.com/v3/repos/hooks/#webhook-headers
func Test_GitHubSource(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedGithub, &called)
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
	req.Header.Add("X-Hub-Signature", xHubSignature(payload, loadFixture(t, "github_key"))) // <-- same as in the endpoint

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)

	called = false
	// Now using form encoded
	form := url.Values{}
	form.Add("payload", string(payload))
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, strings.NewReader(form.Encode()))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Github-Event", "push")
	req.Header.Add("X-Hub-Signature", xHubSignature([]byte(form.Encode()), loadFixture(t, "github_key"))) // <-- same as in the endpoint

	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)

	// check that a bogus signature is rejected
	called = false
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-GitHub-Event", "push")
	req.Header.Add("X-Hub-Signature", xHubSignature(payload[1:] /* <-- i.e., not the same */, loadFixture(t, "github_key")))
	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, 401, res.StatusCode)
}

// xHubSignature generates the X-Hub-Signature header value for the message and key
func xHubSignature(message, key []byte) string {
	mac := hmac.New(sha512.New, key)
	mac.Write(message)
	signature := mac.Sum(nil)

	hexSignature := make([]byte, hex.EncodedLen(len(signature)))
	hex.Encode(hexSignature, signature)
	return "sha512=" + string(hexSignature)
}

// expected notification posted to the flux API. NB because it's a branch head, the refs/heads/ is stripped.
const expectedGitlab = `{"Kind":"git","Source":{"URL":"git@example.com:mike/diaspora.git","Branch":"master"}}`

func Test_GitLabSource(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedGitlab, &called)
	defer downstream.Close()

	endpoint := Endpoint{Source: GitLab, KeyPath: "gitlab_key"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	payload := loadFixture(t, "gitlab_payload")

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	req.Header.Set("X-Gitlab-Token", string(loadFixture(t, "gitlab_key")))

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)

	// Check that bogus token is rejected
	called = false
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	req.Header.Set("X-Gitlab-Token", "BOGUS"+string(loadFixture(t, "gitlab_key")))
	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, 401, res.StatusCode)
}

const expectedHarbor = `{"Kind":"image","Source":{"Name":{"Domain":"demo.goharbor.io","Image":"test123/alpine"}}}`

func Test_Harbor(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedHarbor, &called)
	defer downstream.Close()

	endpoint := Endpoint{Source: Harbor, KeyPath: "harbor_key"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	payload := loadFixture(t, "harbor_payload")

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("Authorization", string(loadFixture(t, "harbor_key")))

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)

	// Check that bogus token is rejected
	called = false
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("Authorization", "BOGUS")
	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, 401, res.StatusCode)
}

const expectedNexus = `{"Kind":"image","Source":{"Name":{"Domain":"container.example.com","Image":"app1/alpine"}}}`

func Test_Nexus(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedNexus, &called)
	defer downstream.Close()

	endpoint := Endpoint{Source: Nexus, KeyPath: "nexus_key", RegistryHost: "container.example.com"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	payload := loadFixture(t, "nexus_payload")

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("X-Nexus-Webhook-Id", "rm:repository:component")
	req.Header.Set("X-Nexus-Webhook-Delivery", "bd9e6aef-0e27-4570-980d-f639c49ab5ed")
	req.Header.Set("X-Nexus-Webhook-Signature", "d06eea380d4631e8c1180b689d10d9ba83ab68f6")

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)

	// check that bogus signature is rejected
	called = false
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("X-Nexus-Webhook-Id", "rm:repository:component")
	req.Header.Set("X-Nexus-Webhook-Delivery", "bd9e6aef-0e27-4570-980d-f639c49ab5ed")
	req.Header.Set("X-Nexus-Webhook-Signature", "BOGUS")
	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, 401, res.StatusCode)
}

const expectedBitbucketCloud = `{"Kind":"git","Source":{"URL":"git@bitbucket.org:mbridgen/dummy.git","Branch":"master"}}`

func Test_BitbucketCloud(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedBitbucketCloud, &called)
	defer downstream.Close()

	endpoint := Endpoint{Source: BitbucketCloud, KeyPath: "bitbucket_cloud_key"}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	payload := loadFixture(t, "bitbucket_cloud_payload")

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-Key", "repo:push")

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)

	// Check that wrong event key gets an error
	called = false
	req, err = http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(payload))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-Key", "flurb")
	res, err = c.Do(req)
	assert.NoError(t, err)
	assert.False(t, called)
	assert.Equal(t, 400, res.StatusCode)
}

func Test_BitbucketServer(t *testing.T) {
	const expected = `{"Kind":"git","Source":{"URL":"ssh://git@bitbucket.redacted.com/~abursavich/hook-test.git","Branch":"master"}}`

	notified := false
	downstream := newDownstream(t, expected, &notified)
	defer downstream.Close()

	endpoint := Endpoint{Source: BitbucketServer, KeyPath: "bitbucket_server_key"}
	digest, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	c := hookServer.Client()
	url := hookServer.URL + "/hook/" + digest
	key := loadFixture(t, "bitbucket_server_key")
	body := loadFixture(t, "bitbucket_server_payload")

	for _, tt := range []struct {
		desc     string
		key      []byte
		body     []byte
		status   int
		notified bool
	}{
		{
			desc:     "ok",
			key:      key,
			body:     body,
			status:   http.StatusOK,
			notified: true,
		},
		{
			desc:   "bad key",
			key:    key[1:],
			body:   body,
			status: http.StatusUnauthorized,
		},
		{
			desc:   "bad payload",
			key:    key,
			body:   body[1:],
			status: http.StatusBadRequest,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			req, err := http.NewRequest("POST", url, bytes.NewReader(tt.body))
			assert.NoError(t, err)
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("X-Event-Key", "repo:refs_changed")
			req.Header.Add("X-Hub-Signature", xHubSignature(tt.body, tt.key))

			notified = false
			resp, err := c.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
			assert.Equal(t, tt.notified, notified)
		})
	}
}

const expectedGoogleContainerRegistry = `{"Kind":"image","Source":{"Name":{"Domain":"us.gcr.io","Image":"am/am.kebab.api"}}}`

// Test that a google pubsub message (push) arriving from Google Container Registry
// calls the downstream with an image update.
// GCR Docs:
// https://cloud.google.com/container-registry/docs/configuring-notifications
// Pubsub Push Docs:
// https://cloud.google.com/pubsub/docs/push
func Test_GoogleContainerRegistry_WhenNoAuth(t *testing.T) {
	var called bool
	downstream := newDownstream(t, expectedGoogleContainerRegistry, &called)
	defer downstream.Close()

	endpoint := Endpoint{Source: GoogleContainerRegistry, KeyPath: "gcr_key", GCR: nil}
	fp, handler, err := HandlerFromEndpoint("test/fixtures", downstream.URL, endpoint)
	assert.NoError(t, err)

	hookServer := httptest.NewTLSServer(handler)
	defer hookServer.Close()

	c := hookServer.Client()
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, bytes.NewReader(loadFixture(t, "gcr_payload")))
	assert.NoError(t, err)

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, res.StatusCode)
	assert.Empty(t, res.Body)
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (s roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return s(r)
}

func Test_GoogleContainerRegistry_WhenAuth(t *testing.T) {
	c := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBuffer(loadFixture(t, "gcr_auth_result"))),
			}, nil
		}),
	}

	token := "Bearer 123"
	audience := "gcr-update"

	err := authenticateRequest(c, token, audience)
	assert.NoError(t, err)
}

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Example from https://docs.docker.com/docker-hub/webhooks/
const dockerhubPayload = `
{
  "callback_url": "https://registry.hub.docker.com/u/svendowideit/testhook/hook/2141b5bi5i5b02bec211i4eeih0242eg11000a/",
  "push_data": {
    "images": [
        "27d47432a69bca5f2700e4dff7de0388ed65f9d3fb1ec645e2bc24c223dc1cc3",
        "51a9c7c1f8bb2fa19bcd09789a34e63f35abb80044bc10196e304f6634cc582c",
        "..."
    ],
    "pushed_at": 1.417566161e+09,
    "pusher": "trustedbuilder",
    "tag": "latest"
  },
  "repository": {
    "comment_count": 0,
    "date_created": 1.417494799e+09,
    "description": "",
    "dockerfile": "#\n# BUILD\u0009\u0009docker build -t svendowideit/apt-cacher .\n# RUN\u0009\u0009docker run -d -p 3142:3142 -name apt-cacher-run apt-cacher\n#\n# and then you can run containers with:\n# \u0009\u0009docker run -t -i -rm -e http_proxy http://192.168.1.2:3142/ debian bash\n#\nFROM\u0009\u0009ubuntu\n\n\nVOLUME\u0009\u0009[/var/cache/apt-cacher-ng]\nRUN\u0009\u0009apt-get update ; apt-get install -yq apt-cacher-ng\n\nEXPOSE \u0009\u00093142\nCMD\u0009\u0009chmod 777 /var/cache/apt-cacher-ng ; /etc/init.d/apt-cacher-ng start ; tail -f /var/log/apt-cacher-ng/*\n",
    "full_description": "Docker Hub based automated build from a GitHub repo",
    "is_official": false,
    "is_private": true,
    "is_trusted": true,
    "name": "testhook",
    "namespace": "svendowideit",
    "owner": "svendowideit",
    "repo_name": "svendowideit/testhook",
    "repo_url": "https://registry.hub.docker.com/u/svendowideit/testhook/",
    "star_count": 0,
    "status": "Active"
  }
}
`

const expectedDockerhub = `{"Kind":"image","Source":{"Name":{"Domain":"","Image":"svendowideit/testhook"}}}`

func newDownstream(t *testing.T, expectedPayload string) *httptest.Server {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		bytes, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedPayload, string(bytes))
		fmt.Fprintln(w, "OK")
	}))
	return downstream
}

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
	req, err := http.NewRequest("POST", hookServer.URL+"/hook/"+fp, strings.NewReader(dockerhubPayload))
	assert.NoError(t, err)

	res, err := c.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

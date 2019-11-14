package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const missingVersion = `
# missing fluxRecvVersion
# all else well
api: http://localhost:3034/fluxapi
endpoints:
- keyPath: ./foo_rsa
- source: DockerHub
`

const completelyDifferentFile = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
spec:
  template:
    spec:
      containers:
      - name: hello
        image: helloworld
`

func TestBadConfigs(t *testing.T) {
	for name, testcase := range map[string]string{
		"missing version":    missingVersion,
		"wrong kind of file": completelyDifferentFile,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ConfigFromBytes([]byte(testcase))
			assert.Error(t, err)
		})
	}
}

const fullConfig = `
fluxRecvVersion: 1
api: http://localhost:3031/api/flux
endpoints:
- source: GitHub
  keyPath: ./github_rsa
- source: DockerHub
  keyPath: ./dockerhub_rsa
`

const minimalConfig = `
fluxRecvVersion: 1
`

func TestGoodConfigs(t *testing.T) {
	for name, testcase := range map[string]string{
		"minimal":     minimalConfig,
		"full config": fullConfig,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ConfigFromBytes([]byte(testcase))
			assert.NoError(t, err)
		})
	}
}

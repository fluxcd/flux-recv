module github.com/fluxcd/flux-recv

go 1.13

require (
	github.com/fluxcd/flux v1.15.0
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-github/v28 v28.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	golang.org/x/sync v0.0.0-20190227155943-e225da77a7e6
	gopkg.in/yaml.v2 v2.2.5 // indirect
)

replace github.com/docker/distribution => github.com/2opremio/distribution v0.0.0-20190419185413-6c9727e5e5de

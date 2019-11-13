.PHONY: all image test

BIN=./build/flux-recv

all: image

image: ${BIN} Dockerfile
	cp Dockerfile ./build/
	docker build -t fluxcd/flux-recv ./build

${BIN}: # deliberately no prereqs; let go figure it out
	go build -mod readonly -o $@ ./cmd/flux-recv/main.go

test:
	go test -mod readonly -v ./cmd/flux-recv

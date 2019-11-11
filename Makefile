.PHONY: all image test

BIN=./build/flux-recv

all: image

image: ${BIN} Dockerfile
	cp Dockerfile ./build/
	docker build -t fluxcd/flux-recv ./build

./build/flux-recv: # deliberately no prereqs; let go figure it out
	go build -o $@ ./cmd/flux-recv/main.go

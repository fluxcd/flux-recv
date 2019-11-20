BIN=./build/flux-recv

.PHONY: all image test bin

all: image

image: ${BIN} Dockerfile
	cp Dockerfile ./build/
	docker build -t fluxcd/flux-recv ./build

bin: ${BIN}

${BIN}: # deliberately no prereqs; let go figure it out
	CGO_ENABLED=0 go build -mod readonly -o $@ .

test:
	CGO_ENABLED=0 go test -mod readonly -v .

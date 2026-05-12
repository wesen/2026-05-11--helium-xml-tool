.PHONY: build test clean lint

BINARY=xml
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
LDFLAGS=-ldflags "-X github.com/go-go-golems/xml/pkg/cmds.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/xml/

test:
	go test ./...

test-integration: build
	./xml --help
	./xml version

clean:
	rm -f $(BINARY)
	go clean ./...

lint:
	golangci-lint run ./...

install: build
	cp $(BINARY) /usr/local/bin/

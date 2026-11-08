BINARY  := bin/rocketchat-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet fmt run run-http clean

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/rocketchat-mcp

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

run:
	go run ./cmd/rocketchat-mcp

run-http:
	go run ./cmd/rocketchat-mcp -transport http -addr :8080

clean:
	rm -rf bin

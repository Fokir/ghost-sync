MODULE  := github.com/sokolovsky/ghost-sync
BINARY  := ghost-sync
CMD     := ./cmd/ghost-sync

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
  -X main.version=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.date=$(DATE)"

.PHONY: build test lint clean

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)

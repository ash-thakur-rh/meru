.PHONY: build install clean test test-unit test-smoke vet fmt fmt-check run dev ui

BINARY   := meru
CMD      := ./cmd/meru
INSTALL  := $(shell go env GOPATH)/bin/$(BINARY)

## Build the web UI and copy into the Go embed path
ui:
	cd web && npm run build
	rm -rf internal/ui/dist
	cp -r web/dist internal/ui/dist

## Build the control-plane binary (includes embedded UI from last `make ui`)
build:
	go build -o $(BINARY) $(CMD)
	go build -o meru-node ./cmd/meru-node

## Build UI + both binaries in one shot
all: ui build

## Install both binaries to GOPATH/bin
install: ui
	go install $(CMD) ./cmd/meru-node
	@echo "Installed → $(INSTALL) + $(shell go env GOPATH)/bin/meru-node"

## Run unit + integration tests (no real binaries required)
test-unit:
	go test ./...

## Run full-stack smoke tests (builds the meru binary)
test-smoke:
	go test -tags smoke ./tests/...

## Run all tests (unit + smoke)
test: test-unit test-smoke

## Format all Go and web source files
fmt:
	go fmt ./...
	cd web && npm run format

## Check formatting without writing (useful in CI)
fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || \
		(echo "Go files need formatting. Run: make fmt"; exit 1)
	cd web && npm run format:check

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf web/dist internal/ui/dist

## Start the daemon (uses last built UI)
run:
	go run $(CMD) serve

## Start Vite dev server (proxies API to :8080)
dev:
	cd web && npm run dev

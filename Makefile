# Go lives at $HOME/.local/go and isn't on PATH by default; prepend it.
export PATH := $(HOME)/.local/go/bin:$(PATH)

GO      ?= go
BINARY  ?= apahe
PKG     := ./...
LDFLAGS := -s -w

.DEFAULT_GOAL := help

## help: list targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

## build: compile the binary
.PHONY: build
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## run: build and run (pass args via ARGS="-s naruto -e 1")
.PHONY: run
run:
	$(GO) run . $(ARGS)

## test: run unit tests
.PHONY: test
test:
	$(GO) test $(PKG)

## test-verbose: run tests with -v
.PHONY: test-verbose
test-verbose:
	$(GO) test -v $(PKG)

## cover: tests with coverage report
.PHONY: cover
cover:
	$(GO) test -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out

## vet: go vet
.PHONY: vet
vet:
	$(GO) vet $(PKG)

## fmt: format all code
.PHONY: fmt
fmt:
	$(GO) fmt $(PKG)

## tidy: sync go.mod/go.sum
.PHONY: tidy
tidy:
	$(GO) mod tidy

## check: fmt + vet + test (pre-commit gate)
.PHONY: check
check: fmt vet test

## install: build and install binary as 'apahe' to GOBIN (or ~/go/bin)
.PHONY: install
install:
	$(GO) build -ldflags "$(LDFLAGS)" -o "$(or $(GOBIN),$(HOME)/go/bin)/$(BINARY)" .

## build-all: cross-compile linux/darwin/windows amd64+arm64 into dist/
.PHONY: build-all
build-all:
	@mkdir -p dist
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; [ $$os = windows ] && ext=".exe"; \
			echo "building dist/$(BINARY)-$$os-$$arch$$ext"; \
			GOOS=$$os GOARCH=$$arch $(GO) build -ldflags "$(LDFLAGS)" \
				-o dist/$(BINARY)-$$os-$$arch$$ext . ; \
		done; \
	done

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY) coverage.out
	rm -rf dist

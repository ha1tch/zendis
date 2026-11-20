# Makefile - zendis, a Z80/Z80N disassembler in Go
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

BINARY  := zendis
PKG     := github.com/ha1tch/zendis
CMD     := ./cmd/zendis
BIN_DIR := build
VERSION := $(shell cat VERSION 2>/dev/null)

GO      := go
GOFLAGS :=

.PHONY: all build test vet fmt smoke install clean version help

all: build

## build: compile the zendis binary into build/
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD)
	@echo "built $(BIN_DIR)/$(BINARY) ($(VERSION))"

## test: run the Go test suite once
test:
	$(GO) test ./... -count=1

## vet: run go vet across the module
vet:
	$(GO) vet ./...

## fmt: format all Go source
fmt:
	$(GO) fmt ./...

## smoke: disassemble a tiny known binary as a quick sanity check
smoke: build
	@echo "-- smoke: disassembling a known snippet"
	@tmp=$$(mktemp); \
	printf '\076\001\311' > "$$tmp"; \
	out=$$($(BIN_DIR)/$(BINARY) "$$tmp" 2>/dev/null); \
	rm -f "$$tmp"; \
	printf '%s\n' "$$out"; \
	echo "$$out" | grep -q 'LD A,0x01' && echo "$$out" | grep -q 'RET' \
		&& echo "   ok" || { echo "   FAIL"; exit 1; }

## install: install zendis into GOBIN / GOPATH bin
install:
	$(GO) install $(GOFLAGS) $(CMD)

## version: print the project version
version:
	@echo "$(VERSION)"

## clean: remove build artefacts
clean:
	rm -rf $(BIN_DIR)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

PROMU := $(shell go env GOPATH)/bin/promu
PREFIX ?= $(shell pwd)
pkgs   = $(shell go list ./... | grep -v /vendor/)

all: format build test

format:
	go fmt $(pkgs)

test:
	go test -short $(pkgs)

build: promu
	@$(PROMU) build --verbose --prefix $(PREFIX)

promu:
	go get -u github.com/prometheus/promu

.PHONY: promu

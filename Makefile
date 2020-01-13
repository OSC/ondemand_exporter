PROMU := $(shell go env GOPATH)/bin/promu

build: promu
	@$(PROMU) build --verbose

promu:
	@GOOS=$(shell uname -s | tr A-Z a-z) \
		GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
		go get -u github.com/prometheus/promu

.PHONY: promu

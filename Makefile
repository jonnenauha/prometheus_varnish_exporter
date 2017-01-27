GHR   := $(GOPATH)/bin/github-release
GO    := GO15VENDOREXPERIMENT=1 go
PROMU := $(GOPATH)/bin/promu
pkgs   = $(shell $(GO) list ./... | grep -v /vendor/)

PREFIX       ?= $(shell pwd)
BIN_DIR      ?= $(shell pwd)
TARBALLS_DIR ?= $(shell pwd)/.tarballs

all: format build test

.PHONY: build
build: | $(PROMU)
	@echo ">> building binaries"
	@$(PROMU) build --prefix $(PREFIX)

.PHONY: crossbuild
crossbuild: | $(PROMU)
	@echo ">> building cross-platform binaries"
	@$(PROMU) crossbuild

.PHONY: format
format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

.PHONY: release
release: | $(GHR) $(PROMU)
	@echo ">> uploading tarballs to the Github release"
	@$(PROMU) release ${TARBALLS_DIR}

.PHONY: style
style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

.PHONY: tarball
tarball: | $(PROMU)
	@echo ">> building release tarball"
	@$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)

.PHONY: tarballs
tarballs: crossbuild | $(PROMU)
	@echo ">> building release tarballs"
	@$(PROMU) crossbuild tarballs

.PHONY: test
test:
	@echo ">> running tests"
	@$(GO) test -short -race $(pkgs)

.PHONY: vet
vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

$(GHR):
	@$(GO) get -u github.com/aktau/github-release

$(PROMU):
	@GOOS=$(shell uname -s | tr A-Z a-z) \
		GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
		$(GO) get -u github.com/prometheus/promu

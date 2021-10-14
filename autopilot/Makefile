GOLANG_IMAGE = golang:1.18.1-alpine
BUILD_IMAGE = autopilot-build:latest
BUILD_GO_CGO_ENABLED = 1
VERSION ?= $(shell git describe --tags)
LD_FLAGS ?= -w -extldflags=-static
LD_FLAGS += -X github.com/k0sproject/autopilot/pkg/build.Version=$(VERSION)
BUILD_GO_FLAGS = -tags='osusergo,netgo' -buildmode=pie -ldflags='$(LD_FLAGS)'

GO ?= GOCACHE=/gocache/build GOMODCACHE=/gocache/mod docker run --rm \
    -v "$(CURDIR)":/go/src \
    -v autopilot.gocache:/gocache \
    -w /go/src \
    -e CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) \
    -e GOCACHE \
    -e GOMODCACHE \
    --user $$(id -u) \
    $(BUILD_IMAGE) go


.PHONY: all
all: autopilot

autopilot: build

.PHONY: build
build: .autopilot.docker-vol.gocache .autopilot.buildimage
	$(GO) build $(BUILD_GO_FLAGS) ./cmd/autopilot

.PHONY: test
test: build
	$(GO) test -count=1 -v ./cmd/... ./pkg/... ./internal/...

.PHONY: inttest
inttest: build
	$(MAKE) -C inttest

.PHONY: clean
clean:
	-rm -f autopilot .autopilot.*
	-docker volume rm autopilot.gocache
	$(MAKE) -C inttest $@

.autopilot.docker-vol.gocache:
	docker volume create autopilot.gocache
	docker run --rm -v autopilot.gocache:/gocache alpine:latest \
	    install -d -o $$(id -u) -g $$(id -g) /gocache/mod /gocache/build

	touch $@

# Temporary, until we can unify the image Dockerfile with this build
define BUILDIMAGE
FROM $(GOLANG_IMAGE) AS build\n
RUN apk add binutils-gold build-base git
endef

export BUILDIMAGE
.autopilot.buildimage:
	echo $$BUILDIMAGE | docker build -t $(BUILD_IMAGE)  -
	touch $@


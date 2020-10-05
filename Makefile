
GO_SRCS := $(shell find . -type f -name '*.go')

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   fetch	fetch precompiled binaries from internet (except kine)
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

GOOS ?= linux
GOARCH ?= amd64

VERSION ?= dev
golint := $(shell which golint)

ifeq ($(golint),)
golint := GO111MODULE=off go get -u golang.org/x/lint/golint && GO111MODULE=off go run golang.org/x/lint/golint
endif

.PHONY: all
all: build

ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
pkg/assets/zz_generated_offsets.go:
	rm -f bindata && touch bindata
	printf "%s\n\n%s\n%s\n" \
		"package assets" \
		"var BinData = map[string]struct{ offset, size int64 }{}" \
		"var BinDataSize int64 = 0" \
		> $@
else
pkg/assets/zz_generated_offsets.go: embedded-bins/staging/linux/bin
	go generate
endif

mke: pkg/assets/zz_generated_offsets.go $(GO_SRCS)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-w -s -X main.Version=$(VERSION)" -o mke.code main.go
	cat mke.code bindata > $@.tmp && chmod +x $@.tmp && mv $@.tmp $@

.PHONY: build
build: mke

.PHONY: bins
bins: .bins.stamp

embedded-bins/staging/linux/bin: .bins.stamp

.bins.stamp:
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE)
	touch $@

.PHONY: lint
lint:
	$(golint) -set_exit_status ./...

.PHONY: check-network
check-network: mke
	$(MAKE) -C tests check

.PHONY: check-basic
check-basic: mke
	$(MAKE) -C inttest check-basic

.PHONY: clean
clean:
	rm -f pkg/assets/zz_generated_offsets.go mke .bins.stamp bindata
	$(MAKE) -C embedded-bins clean



GO_SRCS := $(shell find . -type f -name '*.go')

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   fetch	fetch precompiled binaries from internet (except kine)
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

# mke runs on linux even if its built on mac or windows
GOOS ?= linux
GOARCH ?= $(shell go env GOARCH)
GOPATH ?= $(shell go env GOPATH)

VERSION ?= dev
golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.31.0 && "${GOPATH}/bin/golangci-lint"
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
lint: pkg/assets/zz_generated_offsets.go
	$(golint) run ./...

.PHONY: check-network
check-network: mke
	$(MAKE) -C inttest check-network

.PHONY: check-basic
check-basic: mke
	$(MAKE) -C inttest check-basic

.PHONY: check-basic
check-hacontrolplane: mke
	$(MAKE) -C inttest check-hacontrolplane

.PHONY: check-unit
check-unit: pkg/assets/zz_generated_offsets.go
	go test -race ./pkg/...

.PHONY: clean
clean:
	rm -f pkg/assets/zz_generated_offsets.go mke .bins.stamp bindata
	$(MAKE) -C embedded-bins clean

.PHONY: bindata-manifests
bindata-manifests:
	go-bindata -o static/gen_calico.go -pkg static -prefix static static/...


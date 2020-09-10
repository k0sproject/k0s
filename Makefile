
GO_SRCS := $(shell find -name '*.go')

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   fetch	fetch precompiled binaries from internet (except kine)
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

VERSION ?= dev

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
	CGO_ENABLED=0 go build -ldflags="-w -s -X main.Version=$(VERSION)" -o mke.code main.go
	cat mke.code bindata > $@.tmp && chmod +x $@.tmp && mv $@.tmp $@

.PHONY: build
build: mke

.PHONY: bins
bins: .bins.stamp

embedded-bins/staging/linux/bin: .bins.stamp

.bins.stamp:
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE)
	touch $@

.PHONY: check
check: mke
	$(MAKE) -C tests check

.PHONY: check-network
check-network: mke
	$(MAKE) -C tests check-network

.PHONY: clean
clean:
	rm -f pkg/assets/zz_generated_offsets.go mke .bins.stamp bindata
	$(MAKE) -C embedded-bins clean


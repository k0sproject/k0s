
GO_SRCS := $(shell find . -type f -name '*.go')

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   fetch	fetch precompiled binaries from internet
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

# k0s runs on linux even if its built on mac or windows
GOOS ?= linux
TARGET_OS ?= linux
GOARCH ?= $(shell go env GOARCH)
GOPATH ?= $(shell go env GOPATH)

VERSION ?= $(shell git describe --tags)
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
pkg/assets/zz_generated_offsets.go: # embedded-bins/staging/${TARGET_OS}/bin gen_bindata.go
	# TODO: this dependencies moved here instead of being listed under the actual dependencies because it seems
	# like variable interpolation doesn't work there
	make embedded-bins/staging/${TARGET_OS}/bin gen_bindata.go
	go generate
endif


k0s: export TARGET_OS := linux
k0s: pkg/assets/zz_generated_offsets.go embedded-bins/staging/linux/bin $(GO_SRCS)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags='-w -s -X github.com/k0sproject/k0s/pkg/build.Version=$(VERSION) -X "github.com/k0sproject/k0s/pkg/build.EulaNotice=$(EULA_NOTICE)" -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=$(SEGMENT_TOKEN)' -o k0s.code main.go
	cat k0s.code bindata_${TARGET_OS} > $@.tmp && chmod +x $@.tmp && mv $@.tmp $@

k0s.exe: export TARGET_OS := windows
k0s.exe: pkg/assets/zz_generated_offsets.go embedded-bins/staging/windows/bin $(GO_SRCS)
	CGO_ENABLED=0 GOOS=windows GOARCH=$(GOARCH) go build -ldflags='-w -s -X github.com/k0sproject/k0s/pkg/build.Version=$(VERSION) -X "github.com/k0sproject/k0s/pkg/build.EulaNotice=$(EULA_NOTICE)" -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=$(SEGMENT_TOKEN)' -o k0s.exe.code main.go
	cat k0s.exe.code bindata_${TARGET_OS} > $@.tmp && chmod +x $@.tmp && mv $@.tmp $@

.PHONY: build
build: k0s

.PHONY: bins
bins: .bins.stamp

embedded-bins/staging/linux/bin: .bins.linux.stamp
embedded-bins/staging/windows/bin: .bins.windows.stamp

.bins.windows.stamp:
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE) TARGET_OS=windows
	touch $@

.bins.linux.stamp:
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE) TARGET_OS=linux
	touch $@

.PHONY: lint
lint: pkg/assets/zz_generated_offsets.go
	$(golint) run ./...

smoketests := check-addons check-basic check-byocri check-hacontrolplane check-kine check-network check-singlenode
.PHONY: $(smoketests)
$(smoketests): k0s
	$(MAKE) -C inttest $@

.PHONY: check-unit
check-unit: pkg/assets/zz_generated_offsets.go
	go test -race ./pkg/...

.PHONY: clean
clean:
	rm -f pkg/assets/zz_generated_offsets.go k0s .bins.*stamp bindata*
	$(MAKE) -C embedded-bins clean

.PHONY: manifests
manifests:
	controller-gen crd paths="./..." output:crd:artifacts:config=static/manifests/helm/CustomResourceDefinition object

.PHONY: bindata-manifests
bindata-manifests:
	go-bindata -o static/gen_manifests.go -pkg static -prefix static static/...

.PHONY: generate-bindata
generate-bindata:
	GOOS=${GOHOSTOS} go run gen_bindata.go -o bindata_${TARGET_OS} -pkg assets -gofile pkg/assets/zz_generated_offsets.go -prefix embedded-bins/staging/${TARGET_OS}/ embedded-bins/staging/${TARGET_OS}/bin

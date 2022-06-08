include embedded-bins/Makefile.variables
include inttest/Makefile.variables

GO_SRCS := $(shell find . -type f -name '*.go' -not -path './build/cache/*' -not -path './inttest/*' -not -name '*_test.go' -not -name 'zz_generated*')
GO_DIRS := . ./cmd/... ./pkg/... ./internal/... ./static/... ./hack/...

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

# k0s runs on linux even if its built on mac or windows
TARGET_OS ?= linux
GOARCH ?= $(shell go env GOARCH)
GOPATH ?= $(shell go env GOPATH)
BUILD_UID ?= $(shell id -u)
BUILD_GID ?= $(shell id -g)
BUILD_GO_FLAGS := -tags osusergo
BUILD_GO_CGO_ENABLED ?= 0
BUILD_GO_LDFLAGS_EXTRA :=
DEBUG ?= false

VERSION ?= $(shell git describe --tags)
ifeq ($(DEBUG), false)
LD_FLAGS ?= -w -s
endif

KUBECTL_VERSION = $(shell go mod graph |  grep "github.com/k0sproject/k0s" |  grep kubectl  | cut -d "@" -f 2 | sed "s/v0\./1./")
KUBECTL_MAJOR= $(shell echo ${KUBECTL_VERSION} | cut -d "." -f 1)
KUBECTL_MINOR= $(shell echo ${KUBECTL_VERSION} | cut -d "." -f 2)

# https://reproducible-builds.org/docs/source-date-epoch/#makefile
# https://reproducible-builds.org/docs/source-date-epoch/#git
# https://stackoverflow.com/a/15103333
BUILD_DATE_FMT = %Y-%m-%dT%H:%M:%SZ
ifdef SOURCE_DATE_EPOCH
	BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "+$(BUILD_DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "+$(BUILD_DATE_FMT)" 2>/dev/null || date -u "+$(BUILD_DATE_FMT)")
else
	BUILD_DATE ?= $(shell TZ=UTC git log -1 --pretty=%cd --date='format-local:$(BUILD_DATE_FMT)' || date -u +$(BUILD_DATE_FMT))
endif

LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.Version=$(VERSION)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.RuncVersion=$(runc_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.ContainerdVersion=$(containerd_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KubernetesVersion=$(kubernetes_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KineVersion=$(kine_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.EtcdVersion=$(etcd_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KonnectivityVersion=$(konnectivity_version)
LD_FLAGS += -X "github.com/k0sproject/k0s/pkg/build.EulaNotice=$(EULA_NOTICE)"
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=$(SEGMENT_TOKEN)
LD_FLAGS += -X k8s.io/component-base/version.gitVersion=v$(KUBECTL_VERSION)
LD_FLAGS += -X k8s.io/component-base/version.gitMajor=$(KUBECTL_MAJOR)
LD_FLAGS += -X k8s.io/component-base/version.gitMinor=$(KUBECTL_MINOR)
LD_FLAGS += -X k8s.io/component-base/version.buildDate=$(BUILD_DATE)
LD_FLAGS += -X k8s.io/component-base/version.gitCommit=not_available
LD_FLAGS += $(BUILD_GO_LDFLAGS_EXTRA)

golint := $(shell which golangci-lint 2>/dev/null)
ifeq ($(golint),)
golint := cd hack/ci-deps && go install github.com/golangci/golangci-lint/cmd/golangci-lint && cd ../.. && "${GOPATH}/bin/golangci-lint"
endif

go_bindata := $(shell which go-bindata 2>/dev/null)
ifeq ($(go_bindata),)
go_bindata := cd hack/ci-deps && go install github.com/kevinburke/go-bindata/... && cd ../.. && "${GOPATH}/bin/go-bindata"
endif

go_clientgen := $(shell which client-gen 2>/dev/null)
ifeq ($(go_clientgen),)
go_clientgen := cd hack/ci-deps && go install k8s.io/code-generator/cmd/client-gen@v0.22.2 && cd ../.. && "${GOPATH}/bin/client-gen"
endif

go_controllergen := $(shell which controller-gen 2>/dev/null)
ifeq ($(go_controllergen),)
go_controllergen := cd hack/ci-deps && go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.8.0 && cd ../.. && "${GOPATH}/bin/controller-gen"
endif

GOLANG_IMAGE = golang:$(go_version)-alpine3.16
GO ?= GOCACHE=/go/src/github.com/k0sproject/k0s/build/cache/go/build GOMODCACHE=/go/src/github.com/k0sproject/k0s/build/cache/go/mod docker run --rm \
	-v "$(CURDIR)":/go/src/github.com/k0sproject/k0s \
	-w /go/src/github.com/k0sproject/k0s \
	-e GOOS \
	-e CGO_ENABLED \
	-e GOARCH \
	-e GOCACHE \
	-e GOMODCACHE \
	--user $(BUILD_UID):$(BUILD_GID) \
	$(GOLANG_IMAGE) go

.PHONY: build
ifeq ($(TARGET_OS),windows)
build: k0s.exe
else
build: k0s
endif

.k0sbuild.docker-image.k0s: build/Dockerfile
	docker build --rm \
		--build-arg BUILDIMAGE=golang:$(go_version)-alpine3.16 \
		-f build/Dockerfile \
		-t k0sbuild.docker-image.k0s build/
	touch $@

.PHONY: all
all: k0s k0s.exe

go.sum: go.mod .k0sbuild.docker-image.k0s
	$(GO) mod tidy

codegen_targets = \
	static/gen_manifests.go \
	pkg/assets/zz_generated_offsets_$(TARGET_OS).go

zz_os = $(patsubst pkg/assets/zz_generated_offsets_%.go,%,$@)
print_empty_generated_offsets = printf "%s\n\n%s\n%s\n" \
			"package assets" \
			"var BinData = map[string]struct{ offset, size int64 }{}" \
			"var BinDataSize int64"
ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go:
	rm -f bindata_$(zz_os) && touch bindata_$(zz_os)
	$(print_empty_generated_offsets) > $@
else
pkg/assets/zz_generated_offsets_linux.go: .bins.linux.stamp
pkg/assets/zz_generated_offsets_windows.go: .bins.windows.stamp
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go: .k0sbuild.docker-image.k0s go.sum
	GOOS=${GOHOSTOS} $(GO) run hack/gen-bindata/main.go -o bindata_$(zz_os) -pkg assets \
	     -gofile pkg/assets/zz_generated_offsets_$(zz_os).go \
	     -prefix embedded-bins/staging/$(zz_os)/ embedded-bins/staging/$(zz_os)/bin
endif

# needed for unit tests on macos
pkg/assets/zz_generated_offsets_darwin.go:
	$(print_empty_generated_offsets) > $@

k0s: TARGET_OS = linux
k0s: BUILD_GO_CGO_ENABLED = 1
k0s: GOLANG_IMAGE = "k0sbuild.docker-image.k0s"
k0s: BUILD_GO_LDFLAGS_EXTRA = -extldflags=-static
k0s: .k0sbuild.docker-image.k0s

k0s.exe: TARGET_OS = windows
k0s.exe: BUILD_GO_CGO_ENABLED = 0
k0s.exe: GOLANG_IMAGE = golang:$(go_version)-alpine3.16

k0s.exe k0s: $(codegen_targets)

k0s.exe k0s: $(GO_SRCS) go.sum
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) GOOS=$(TARGET_OS) GOARCH=$(GOARCH) $(GO) build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o $@.code main.go
	cat $@.code bindata_$(TARGET_OS) > $@.tmp \
		&& rm -f $@.code \
		&& chmod +x $@.tmp \
		&& mv $@.tmp $@

.bins.windows.stamp .bins.linux.stamp: embedded-bins/Makefile.variables
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE) TARGET_OS=$(patsubst .bins.%.stamp,%,$@)
	touch $@

.PHONY: codegen
codegen: $(codegen_targets)

.PHONY: lint
lint: go.sum codegen
	$(golint) run --verbose $(GO_DIRS)

.PHONY: $(smoketests)
check-airgap: image-bundle/bundle.tar
$(smoketests): k0s
	$(MAKE) -C inttest $@

.PHONY: smoketests
smoketests:  $(smoketests)

.PHONY: check-unit
check-unit: GO_TEST_RACE ?= -race
check-unit: go.sum codegen
	$(GO) test $(GO_TEST_RACE) `$(GO) list $(GO_DIRS)`

.PHONY: check-image-validity
check-image-validity: go.sum
	$(GO) run hack/validate-images/main.go -architectures amd64,arm64,arm

check-unit \
check-image-validity \
clean-gocache: GO = \
  GOCACHE='$(CURDIR)/build/cache/go/build' \
  GOMODCACHE='$(CURDIR)/build/cache/go/mod' \
  go

.PHONY: clean-gocache
clean-gocache:
	$(GO) clean -cache -modcache

clean-docker-image:
	-docker rmi k0sbuild.docker-image.k0s -f
	-rm -f .k0sbuild.docker-image.k0s

.PHONY: clean
clean: clean-gocache clean-docker-image
	-rm -f pkg/assets/zz_generated_offsets_*.go k0s k0s.exe .bins.*stamp bindata* static/gen_manifests.go
	-$(MAKE) -C docs clean
	-$(MAKE) -C embedded-bins clean
	-$(MAKE) -C image-bundle clean
	-$(MAKE) -C inttest clean

.PHONY: manifests

ROOT_DIR := $(shell pwd)

manifests: .helmCRD .cfgCRD

.helmCRD:
	$(go_controllergen) crd paths="./pkg/apis/helm.k0sproject.io/..." output:crd:artifacts:config=$(ROOT_DIR)/static/manifests/helm/CustomResourceDefinition object

.cfgCRD:
	$(go_controllergen) crd paths="./pkg/apis/k0s.k0sproject.io/v1beta1/..." output:crd:artifacts:config=$(ROOT_DIR)/static/manifests/v1beta1/CustomResourceDefinition object

static/gen_manifests.go: $(shell find static/manifests -type f)
	$(go_bindata) -o static/gen_manifests.go -pkg static -prefix static static/...

.PHONY: generate-bindata
generate-bindata: pkg/assets/zz_generated_offsets_$(TARGET_OS).go

.PHONY: generate-APIClient

generate-APIClient: hack/client-gen/boilerplate.go.txt
	$(go_clientgen) --go-header-file hack/client-gen/boilerplate.go.txt --input="k0s.k0sproject.io/v1beta1" --input-base github.com/k0sproject/k0s/pkg/apis --clientset-name="clientset" -p github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/

image-bundle/image.list: k0s
	./k0s airgap list-images > image-bundle/image.list

image-bundle/bundle.tar: image-bundle/image.list
	$(MAKE) -C image-bundle bundle.tar

.PHONY: docs
docs:
	$(MAKE) -C docs

DOCS_DEV_PORT = 8000

.PHONY: docs-serve-dev
docs-serve-dev:
	$(MAKE) -C docs .docker-image.serve-dev.stamp
	docker run --rm \
	  -v "$(CURDIR):/docs:ro" \
	  -w /docs \
	  -p '$(DOCS_DEV_PORT):8000' \
	  k0sdocs.docker-image.serve-dev

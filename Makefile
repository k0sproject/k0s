include embedded-bins/Makefile.variables
include inttest/Makefile.variables

GO_SRCS := $(shell find . -type f -name '*.go' -a ! -name 'zz_generated*')

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   fetch	fetch precompiled binaries from internet
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

# k0s runs on linux even if its built on mac or windows
TARGET_OS ?= linux
GOARCH ?= $(shell go env GOARCH)
GOPATH ?= $(shell go env GOPATH)
BUILD_GO_FLAGS :=
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
BUILD_DATE = $(shell date ${SOURCE_DATE_EPOCH:+"--date=@${SOURCE_DATE_EPOCH:-}"} -u +'%Y-%m-%dT%H:%M:%SZ')

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
LD_FLAGS += -X k8s.io/component-base/version.gitCommit="not_available"
LD_FLAGS += $(BUILD_GO_LDFLAGS_EXTRA)

golint := $(shell which golangci-lint)
ifeq ($(golint),)
golint := cd hack/ci-deps && go install github.com/golangci/golangci-lint/cmd/golangci-lint && cd ../.. && "${GOPATH}/bin/golangci-lint"
endif

go_bindata := $(shell which go-bindata)
ifeq ($(go_bindata),)
go_bindata := cd hack/ci-deps && go install github.com/kevinburke/go-bindata/... && cd ../.. && "${GOPATH}/bin/go-bindata"
endif

GOLANG_IMAGE = golang:1.16-alpine
GO ?= GOCACHE=/tmp/.cache docker run --rm -v "$(CURDIR)":/go/src/github.com/k0sproject/k0s \
	-w /go/src/github.com/k0sproject/k0s \
	-e GOOS \
	-e CGO_ENABLED \
	-e GOARCH \
	-e GOCACHE \
	--user $$(id -u) \
	$(GOLANG_IMAGE) go

.PHONY: build
ifeq ($(TARGET_OS),windows)
build: k0s.exe
else
build: k0s
endif

.k0sbuild.docker-image.k0s:
	docker build --rm -t k0sbuild.docker-image.k0s -f build/Dockerfile .
	touch $@

.k0sbuild.docker-image.k0s: build/Dockerfile

.PHONY: all
all: k0s k0s.exe

zz_os = $(patsubst pkg/assets/zz_generated_offsets_%.go,%,$@)
ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go:
	rm -f bindata_$(zz_os) && touch bindata_$(zz_os)
	printf "%s\n\n%s\n%s\n" \
		"package assets" \
		"var BinData = map[string]struct{ offset, size int64 }{}" \
		"var BinDataSize int64 = 0" \
		> $@
else
pkg/assets/zz_generated_offsets_linux.go: .bins.linux.stamp
pkg/assets/zz_generated_offsets_windows.go: .bins.windows.stamp
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go: .k0sbuild.docker-image.k0s
	GOOS=${GOHOSTOS} $(GO) run hack/gen-bindata/main.go -o bindata_$(zz_os) -pkg assets \
	     -gofile pkg/assets/zz_generated_offsets_$(zz_os).go \
	     -prefix embedded-bins/staging/$(zz_os)/ embedded-bins/staging/$(zz_os)/bin
endif


k0s: TARGET_OS = linux
k0s: pkg/assets/zz_generated_offsets_linux.go
k0s: BUILD_GO_CGO_ENABLED = 1
k0s: GOLANG_IMAGE = "k0sbuild.docker-image.k0s"
k0s: BUILD_GO_LDFLAGS_EXTRA = -extldflags=-static
k0s: .k0sbuild.docker-image.k0s

k0s.exe: TARGET_OS = windows
k0s.exe: BUILD_GO_CGO_ENABLED = 0
k0s.exe: GOLANG_IMAGE = golang:1.16-alpine
k0s.exe: pkg/assets/zz_generated_offsets_windows.go

k0s.exe k0s: static/gen_manifests.go

k0s.exe k0s: $(GO_SRCS)
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) GOOS=$(TARGET_OS) GOARCH=$(GOARCH) $(GO) build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o $@.code main.go
	cat $@.code bindata_$(TARGET_OS) > $@.tmp \
		&& rm -f $@.code \
		&& chmod +x $@.tmp \
		&& mv $@.tmp $@

.bins.windows.stamp .bins.linux.stamp: embedded-bins/Makefile.variables
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE) TARGET_OS=$(patsubst .bins.%.stamp,%,$@)
	touch $@

SKIP_GOMOD_LINT ?= false
ifeq ($(SKIP_GOMOD_LINT), false)
GOMODLINT=lint-gomod
endif

.PHONY: lint
lint: pkg/assets/zz_generated_offsets_$(TARGET_OS).go ${GOMODLINT}
	$(golint) run ./...

.PHONY: $(smoketests)
check-airgap: image-bundle/bundle.tar
$(smoketests): k0s
	$(MAKE) -C inttest $@

.PHONY: smoketests
smoketests:  $(smoketests)


.PHONY: check-unit
check-unit: pkg/assets/zz_generated_offsets_$(TARGET_OS).go static/gen_manifests.go
	go test -race ./pkg/... ./internal/...

.PHONY: clean
clean:
	-rm -f pkg/assets/zz_generated_offsets_*.go k0s k0s.exe .bins.*stamp bindata* static/gen_manifests.go .k0sbuild.docker-image.k0s
	-docker rmi k0sbuild.docker-image.k0s -f
	-$(MAKE) -C embedded-bins clean
	-$(MAKE) -C image-bundle clean
	-$(MAKE) -C inttest clean

.PHONY: manifests
manifests:
	controller-gen crd paths="./..." output:crd:artifacts:config=static/manifests/helm/CustomResourceDefinition object

static/gen_manifests.go: $(shell find static/manifests -type f)
	$(go_bindata) -o static/gen_manifests.go -pkg static -prefix static static/...

.PHONY: generate-bindata
generate-bindata: pkg/assets/zz_generated_offsets_$(TARGET_OS).go

image-bundle/image.list: k0s
	./k0s airgap list-images > image-bundle/image.list

image-bundle/bundle.tar: image-bundle/image.list
	$(MAKE) -C image-bundle bundle.tar


GOMODTIDYLINT=sh -c '\
if [ `git diff go.mod go.sum | wc -l` -gt "0" ]; then \
	echo "Run \`go mod tidy\` and commit the result"; \
	exit 1; \
fi ; \
${GO} mod tidy; \
if [ `git diff go.mod go.sum | wc -l` -gt "0" ]; then \
 git checkout go.mod go.sum ; \
 echo "Linter failure: go.mod and go.sum have unused deps. Run \`go mod tidy\` and commit the result"; \
 exit 2; \
fi \
 ; ' GOMODTIDYLINT

lint-gomod:
	@${GOMODTIDYLINT}


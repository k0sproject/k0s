include embedded-bins/Makefile.variables
include inttest/Makefile.variables
include hack/tools/Makefile.variables

GO_SRCS := $(shell find . -type f -name '*.go' -not -path './build/cache/*' -not -path './inttest/*' -not -name '*_test.go' -not -name 'zz_generated*')
GO_DIRS := . ./cmd/... ./pkg/... ./internal/... ./static/... ./hack/...

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker

# k0s runs on linux even if its built on mac or windows
TARGET_OS ?= linux
BUILD_UID ?= $(shell id -u)
BUILD_GID ?= $(shell id -g)
BUILD_GO_FLAGS := -tags osusergo
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

GO_ENV ?= docker run --rm \
	-v '$(CURDIR)/build/cache':/run/k0s-build \
	-v '$(CURDIR)':/go/src/github.com/k0sproject/k0s \
	-w /go/src/github.com/k0sproject/k0s \
	-e GOOS \
	-e CGO_ENABLED \
	-e GOARCH \
	--user $(BUILD_UID):$(BUILD_GID) \
	k0sbuild.docker-image.k0s
GO ?= $(GO_ENV) go

.PHONY: all
all: k0s k0s.exe

.PHONY: build
ifeq ($(TARGET_OS),windows)
build: k0s.exe
else
build: k0s
endif

build/cache:
	mkdir -p -- '$@'

.k0sbuild.docker-image.k0s: build/Dockerfile embedded-bins/Makefile.variables | build/cache
	docker build --rm \
		--build-arg BUILDIMAGE=golang:$(go_version)-alpine3.16 \
		-f build/Dockerfile \
		-t k0sbuild.docker-image.k0s build/
	touch $@

go.sum: go.mod .k0sbuild.docker-image.k0s
	$(GO) mod tidy && touch -c -- '$@'

codegen_targets += pkg/apis/helm.k0sproject.io/v1beta1/.controller-gen.stamp
pkg/apis/helm.k0sproject.io/v1beta1/.controller-gen.stamp: $(shell find pkg/apis/helm.k0sproject.io/v1beta1/ -type f -name \*.go)
pkg/apis/helm.k0sproject.io/v1beta1/.controller-gen.stamp: gen_output_dir = helm

codegen_targets += pkg/apis/k0s.k0sproject.io/v1beta1/.controller-gen.stamp
pkg/apis/k0s.k0sproject.io/v1beta1/.controller-gen.stamp: $(shell find pkg/apis/k0s.k0sproject.io/v1beta1 -type f -name \*.go)
pkg/apis/k0s.k0sproject.io/v1beta1/.controller-gen.stamp: gen_output_dir = v1beta1

pkg/apis/%/.controller-gen.stamp: .k0sbuild.docker-image.k0s hack/tools/Makefile.variables
	CGO_ENABLED=0 $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(controller-gen_version)
	$(GO_ENV) controller-gen \
	  crd \
	  paths="./$(dir $@)..." \
	  output:crd:artifacts:config=./static/manifests/$(gen_output_dir)/CustomResourceDefinition \
	  object
	touch -- '$@'

codegen_targets += pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp
pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp: .k0sbuild.docker-image.k0s hack/tools/boilerplate.go.txt embedded-bins/Makefile.variables
pkg/apis/k0s.k0sproject.io/v1beta1/.client-gen.stamp: $(shell find pkg/apis/k0s.k0sproject.io -type f -name \*.go)
	CGO_ENABLED=0 $(GO) install k8s.io/code-generator/cmd/client-gen@v$(patsubst 1.%,0.%,$(kubernetes_version))
	$(GO_ENV) client-gen \
	  --go-header-file hack/tools/boilerplate.go.txt \
	  --input=k0s.k0sproject.io/v1beta1 \
	  --input-base github.com/k0sproject/k0s/pkg/apis \
	  --clientset-name=clientset \
	  --output-package=github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/
	touch -- '$@'

codegen_targets += static/gen_manifests.go
static/gen_manifests.go: .k0sbuild.docker-image.k0s hack/tools/Makefile.variables
static/gen_manifests.go: $(shell find static/manifests -type f)
	CGO_ENABLED=0 $(GO) install github.com/kevinburke/go-bindata/go-bindata@v$(go-bindata_version)
	$(GO_ENV) go-bindata -o static/gen_manifests.go -pkg static -prefix static static/...

codegen_targets += pkg/assets/zz_generated_offsets_$(TARGET_OS).go
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
k0s: BUILD_GO_LDFLAGS_EXTRA = -extldflags=-static
k0s: .k0sbuild.docker-image.k0s

k0s.exe: TARGET_OS = windows
k0s.exe: BUILD_GO_CGO_ENABLED = 0
k0s.exe: GOLANG_IMAGE = golang:$(go_version)-alpine3.16

k0s.exe k0s: $(GO_SRCS) $(codegen_targets) go.sum
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) GOOS=$(TARGET_OS) $(GO) build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o $@.code main.go
	cat $@.code bindata_$(TARGET_OS) > $@.tmp \
		&& rm -f $@.code \
		&& chmod +x $@.tmp \
		&& mv $@.tmp $@

.bins.windows.stamp .bins.linux.stamp: embedded-bins/Makefile.variables
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE) TARGET_OS=$(patsubst .bins.%.stamp,%,$@)
	touch $@

.PHONY: codegen
codegen: $(codegen_targets)

# bindata contains the parts of codegen which aren't version controlled.
.PHONY: bindata
bindata: static/gen_manifests.go pkg/assets/zz_generated_offsets_$(TARGET_OS).go

.PHONY: lint
lint: .k0sbuild.docker-image.k0s go.sum codegen
	CGO_ENABLED=0 $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(golangci-lint_version)
	$(GO_ENV) golangci-lint run --verbose $(GO_DIRS)

.PHONY: $(smoketests)
check-airgap: image-bundle/bundle.tar
$(smoketests): k0s
	$(MAKE) -C inttest $@

.PHONY: smoketests
smoketests: $(smoketests)

.PHONY: check-unit
check-unit: GO_TEST_RACE ?= -race
check-unit: go.sum codegen
	$(GO) test $(GO_TEST_RACE) `$(GO) list $(GO_DIRS)`

.PHONY: check-image-validity
check-image-validity: go.sum
	$(GO) run hack/validate-images/main.go -architectures amd64,arm64,arm

.PHONY: clean-gocache
clean-gocache:
	-find build/cache/go/mod -type d -exec chmod u+w '{}' \;
	rm -rf build/cache/go

clean-docker-image:
	-docker rmi k0sbuild.docker-image.k0s -f
	-rm -f .k0sbuild.docker-image.k0s

.PHONY: clean
clean: clean-gocache clean-docker-image
	-rm -f pkg/assets/zz_generated_offsets_*.go k0s k0s.exe .bins.*stamp bindata* static/gen_manifests.go
	-rm -rf build/cache 
	-find pkg/apis -type f \( -name .client-gen.stamp -or -name .controller-gen.stamp \) -delete
	-$(MAKE) -C docs clean
	-$(MAKE) -C embedded-bins clean
	-$(MAKE) -C image-bundle clean
	-$(MAKE) -C inttest clean

.PHONY: manifests
manifests: .helmCRD .cfgCRD

.PHONY: .helmCRD

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

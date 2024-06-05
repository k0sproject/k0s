include embedded-bins/Makefile.variables
include inttest/Makefile.variables
include hack/tools/Makefile.variables

ifndef HOST_ARCH
HOST_HARDWARE := $(shell uname -m)
ifneq (, $(filter $(HOST_HARDWARE), aarch64 arm64))
  HOST_ARCH := arm64
else ifneq (, $(filter $(HOST_HARDWARE), armv8l armv7l arm))
  HOST_ARCH := arm
else
  ifeq (, $(filter $(HOST_HARDWARE), x86_64 amd64 x64))
    $(warning unknown machine hardware name $(HOST_HARDWARE), assuming amd64)
  endif
  HOST_ARCH := amd64
endif
endif

K0S_GO_BUILD_CACHE ?= build/cache

GO_SRCS := $(shell find . -type f -name '*.go' -not -path './$(K0S_GO_BUILD_CACHE)/*' -not -path './inttest/*' -not -name '*_test.go' -not -name 'zz_generated*')
GO_CHECK_UNIT_DIRS := . ./cmd/... ./pkg/... ./internal/... ./static/... ./hack/...

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= docker
# k0s runs on linux even if it's built on mac or windows
TARGET_OS ?= linux
BUILD_UID ?= $(shell id -u)
BUILD_GID ?= $(shell id -g)
BUILD_GO_TAGS ?= osusergo
BUILD_GO_FLAGS = -tags=$(subst $(space),$(comma),$(BUILD_GO_TAGS)) -buildvcs=false -trimpath
BUILD_CGO_CFLAGS :=
BUILD_GO_LDFLAGS_EXTRA :=
DEBUG ?= false

VERSION ?= $(shell git describe --tags)
ifeq ($(DEBUG), false)
LD_FLAGS ?= -w -s
endif

# https://reproducible-builds.org/docs/source-date-epoch/#makefile
# https://reproducible-builds.org/docs/source-date-epoch/#git
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct || date -u +%s)
BUILD_DATE_FMT = %Y-%m-%dT%H:%M:%SZ
BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "+$(BUILD_DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "+$(BUILD_DATE_FMT)" 2>/dev/null || date -u "+$(BUILD_DATE_FMT)")

LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.Version=$(VERSION)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.RuncVersion=$(runc_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.ContainerdVersion=$(containerd_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KubernetesVersion=$(kubernetes_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KineVersion=$(kine_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.EtcdVersion=$(etcd_version)
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/build.KonnectivityVersion=$(konnectivity_version)
LD_FLAGS += -X "github.com/k0sproject/k0s/pkg/build.EulaNotice=$(EULA_NOTICE)"
LD_FLAGS += -X github.com/k0sproject/k0s/pkg/telemetry.segmentToken=$(SEGMENT_TOKEN)
LD_FLAGS += -X k8s.io/component-base/version.gitVersion=v$(kubernetes_version)
LD_FLAGS += -X k8s.io/component-base/version.gitMajor=$(shell echo '$(kubernetes_version)' | cut -d. -f1)
LD_FLAGS += -X k8s.io/component-base/version.gitMinor=$(shell echo '$(kubernetes_version)' | cut -d. -f2)
LD_FLAGS += -X k8s.io/component-base/version.buildDate=$(BUILD_DATE)
LD_FLAGS += -X k8s.io/component-base/version.gitCommit=not_available
LD_FLAGS += -X github.com/containerd/containerd/version.Version=$(containerd_version)
ifeq ($(EMBEDDED_BINS_BUILDMODE), docker)
ifeq ($(TARGET_OS),linux)
LD_FLAGS += -X github.com/containerd/containerd/version.Revision=$(shell ./embedded-bins/staging/linux/bin/containerd --version | awk '{print $$4}')
endif
endif
LD_FLAGS += $(BUILD_GO_LDFLAGS_EXTRA)

GOLANG_IMAGE ?= $(golang_buildimage)
K0S_GO_BUILD_CACHE_VOLUME_PATH=$(realpath $(K0S_GO_BUILD_CACHE))
GO_ENV ?= docker run --rm \
	-v '$(K0S_GO_BUILD_CACHE_VOLUME_PATH)':/run/k0s-build \
	-v '$(CURDIR)':/go/src/github.com/k0sproject/k0s \
	-w /go/src/github.com/k0sproject/k0s \
	-e GOOS \
	-e CGO_ENABLED \
	-e CGO_CFLAGS \
	-e GOARCH \
	--user $(BUILD_UID):$(BUILD_GID) \
	-- '$(shell cat .k0sbuild.docker-image.k0s)'
GO ?= $(GO_ENV) go

# https://www.gnu.org/software/make/manual/make.html#index-spaces_002c-in-variable-values
nullstring :=
space := $(nullstring) # space now holds a single space
comma := ,

.DELETE_ON_ERROR:

.PHONY: build
ifeq ($(TARGET_OS),windows)
build: k0s.exe
else
BUILD_GO_LDFLAGS_EXTRA = -extldflags=-static
build: k0s
endif

.PHONY: all
all: k0s k0s.exe

$(K0S_GO_BUILD_CACHE):
	mkdir -p -- '$@'

.k0sbuild.docker-image.k0s: build/Dockerfile embedded-bins/Makefile.variables | $(K0S_GO_BUILD_CACHE)
	docker build --progress=plain --iidfile '$@' \
	  --build-arg BUILDIMAGE=$(GOLANG_IMAGE) \
	  -t k0sbuild.docker-image.k0s - <build/Dockerfile

go.sum: go.mod .k0sbuild.docker-image.k0s
	$(GO) mod tidy && touch -c -- '$@'

# List of all the custom APIs that k0s defines.
api_group_versions := $(foreach path,$(wildcard pkg/apis/*/v*/doc.go),$(path:pkg/apis/%/doc.go=%))

# Run controller-gen for all API group versions.
codegen_targets := $(foreach gv,$(api_group_versions),pkg/apis/$(gv)/.controller-gen.stamp)
$(foreach gv,$(api_group_versions),$(eval pkg/apis/$(gv)/.controller-gen.stamp: $$(shell find pkg/apis/$(gv)/ -maxdepth 1 -type f -name '*.go' -not -name '*_test.go' -not -name 'zz_generated*')))
$(foreach gv,$(api_group_versions),pkg/apis/$(gv)/.controller-gen.stamp): .k0sbuild.docker-image.k0s hack/tools/boilerplate.go.txt hack/tools/Makefile.variables
	rm -rf 'static/manifests/$(dir $(@:pkg/apis/%/.controller-gen.stamp=%))CustomResourceDefinition'
	mkdir -p 'static/manifests/$(dir $(@:pkg/apis/%/.controller-gen.stamp=%))'
	gendir="$$(mktemp -d .controller-gen.XXXXXX.tmp)" \
	  && trap "rm -rf -- $$gendir" INT EXIT \
	  && CGO_ENABLED=0 $(GO) run sigs.k8s.io/controller-tools/cmd/controller-gen@v$(controller-gen_version) \
	    paths="./$(dir $@)..." \
	    object:headerFile=hack/tools/boilerplate.go.txt output:object:dir="$$gendir" \
	    crd output:crd:dir='static/manifests/$(dir $(@:pkg/apis/%/.controller-gen.stamp=%))CustomResourceDefinition' \
	  && mv -f -- "$$gendir"/zz_generated.deepcopy.go '$(dir $@).'
	touch -- '$@'

# Generate the k0s client-go clientset based on all custom API group versions.
clientset_input_dirs := $(foreach gv,$(api_group_versions),pkg/apis/$(gv))
codegen_targets += pkg/client/clientset/.client-gen.stamp
pkg/client/clientset/.client-gen.stamp: $(shell find $(clientset_input_dirs) -type f -name '*.go' -not -name '*_test.go' -not -name 'zz_generated*')
pkg/client/clientset/.client-gen.stamp: .k0sbuild.docker-image.k0s hack/tools/boilerplate.go.txt embedded-bins/Makefile.variables
	gendir="$$(mktemp -d .client-gen.XXXXXX.tmp)" \
	  && trap "rm -rf -- $$gendir" INT EXIT \
	  && CGO_ENABLED=0 $(GO) run k8s.io/code-generator/cmd/client-gen@v$(kubernetes_version:1.%=0.%) \
	    --go-header-file=hack/tools/boilerplate.go.txt \
	    --input-base='' \
	    --input=$(subst $(space),$(comma),$(clientset_input_dirs:%=github.com/k0sproject/k0s/%)) \
	    --output-pkg=github.com/k0sproject/k0s/pkg/client \
	    --clientset-name=clientset \
	    --output-dir="$$gendir/out" \
	  && { [ ! -e pkg/client/clientset ] || mv -- pkg/client/clientset "$$gendir/old"; } \
	  && mv -f -- "$$gendir/out/clientset" pkg/client/.
	touch -- '$@'

codegen_targets += static/zz_generated_assets.go
static/zz_generated_assets.go: $(foreach gv,$(api_group_versions),pkg/apis/$(gv)/.controller-gen.stamp) # to generate the CRDs into static/manifests/*/CustomResourceDefinition
static/zz_generated_assets.go: $(shell find static/manifests/calico static/manifests/windows static/misc -type f)
static/zz_generated_assets.go: .k0sbuild.docker-image.k0s hack/tools/Makefile.variables
	CGO_ENABLED=0 $(GO) run github.com/kevinburke/go-bindata/go-bindata@v$(go-bindata_version) \
	  -o '$@' -pkg static -prefix static \
	  $(foreach gv,$(api_group_versions),static/manifests/$(dir $(gv))CustomResourceDefinition/...) \
	  static/manifests/calico/... \
	  static/manifests/windows/... \
	  static/misc/...

ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
BUILD_GO_TAGS += noembedbins
else
codegen_targets += pkg/assets/zz_generated_offsets_$(TARGET_OS).go
zz_os = $(patsubst pkg/assets/zz_generated_offsets_%.go,%,$@)
pkg/assets/zz_generated_offsets_linux.go: .bins.linux.stamp
pkg/assets/zz_generated_offsets_windows.go: .bins.windows.stamp
pkg/assets/zz_generated_offsets_linux.go pkg/assets/zz_generated_offsets_windows.go: .k0sbuild.docker-image.k0s go.sum
	GOOS=${GOHOSTOS} $(GO) run -tags=hack hack/gen-bindata/cmd/main.go -o bindata_$(zz_os) -pkg assets \
	     -gofile pkg/assets/zz_generated_offsets_$(zz_os).go \
	     -prefix embedded-bins/staging/$(zz_os)/ embedded-bins/staging/$(zz_os)/bin
endif

k0s: TARGET_OS = linux
k0s: BUILD_GO_CGO_ENABLED = 1
k0s: .k0sbuild.docker-image.k0s

k0s.exe: TARGET_OS = windows
k0s.exe: BUILD_GO_CGO_ENABLED = 0

k0s.exe k0s: $(GO_SRCS) $(codegen_targets) go.sum
	rm -f -- '$@'
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) CGO_CFLAGS='$(BUILD_CGO_CFLAGS)' GOOS=$(TARGET_OS) $(GO) build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o '$@' main.go
ifneq ($(EMBEDDED_BINS_BUILDMODE),none)
	cat -- bindata_$(TARGET_OS) >>$@
endif
	@printf '\n%s size: %s\n\n' '$@' "$$(du -sh -- $@ | cut -f1)"

.bins.windows.stamp .bins.linux.stamp: embedded-bins/Makefile.variables
	$(MAKE) -C embedded-bins \
	  TARGET_OS=$(patsubst .bins.%.stamp,%,$@) \
	  SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH)
	touch $@

.PHONY: codegen
codegen: $(codegen_targets)

# bindata contains the parts of codegen which aren't version controlled.
.PHONY: bindata
bindata: static/zz_generated_assets.go
ifneq ($(EMBEDDED_BINS_BUILDMODE),none)
bindata: pkg/assets/zz_generated_offsets_$(TARGET_OS).go
endif

.PHONY: lint-copyright
lint-copyright:
	hack/copyright.sh

.PHONY: lint-go
lint-go: GOLANGCI_LINT_FLAGS ?=
lint-go: .k0sbuild.docker-image.k0s go.sum bindata
	CGO_ENABLED=0 $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(golangci-lint_version)
	CGO_CFLAGS='$(BUILD_CGO_CFLAGS)' $(GO_ENV) golangci-lint run --verbose --build-tags=$(subst $(space),$(comma),$(BUILD_GO_TAGS)) $(GOLANGCI_LINT_FLAGS) $(GO_LINT_DIRS)

.PHONY: lint
lint: lint-copyright lint-go

airgap-images.txt: k0s .k0sbuild.docker-image.k0s
	$(GO_ENV) ./k0s airgap list-images --all > '$@'

airgap-image-bundle-linux-amd64.tar: TARGET_PLATFORM := linux/amd64
airgap-image-bundle-linux-arm64.tar: TARGET_PLATFORM := linux/arm64
airgap-image-bundle-linux-arm.tar:   TARGET_PLATFORM := linux/arm/v7
airgap-image-bundle-linux-amd64.tar \
airgap-image-bundle-linux-arm64.tar \
airgap-image-bundle-linux-arm.tar: .k0sbuild.image-bundler.stamp airgap-images.txt
	docker run --rm -i --privileged \
	  -e TARGET_PLATFORM='$(TARGET_PLATFORM)' \
	  '$(shell cat .k0sbuild.image-bundler.stamp)' < airgap-images.txt > '$@'

.k0sbuild.image-bundler.stamp: hack/image-bundler/* embedded-bins/Makefile.variables
	docker build --progress=plain --iidfile '$@' \
	  --build-arg ALPINE_VERSION=$(alpine_patch_version) \
	  -t k0sbuild.image-bundler -- hack/image-bundler

.PHONY: $(smoketests)
check-airgap check-ap-airgap: airgap-image-bundle-linux-$(HOST_ARCH).tar
$(smoketests): k0s
	$(MAKE) -C inttest K0S_IMAGES_BUNDLE='$(CURDIR)/airgap-image-bundle-linux-$(HOST_ARCH).tar' $@

.PHONY: smoketests
smoketests: $(smoketests)

.PHONY: check-unit
ifneq (, $(filter $(HOST_ARCH), arm))
check-unit: GO_TEST_RACE ?=
else
check-unit: GO_TEST_RACE ?= -race
endif
check-unit: BUILD_GO_TAGS += hack
check-unit: .k0sbuild.docker-image.k0s go.sum bindata
	CGO_CFLAGS='$(BUILD_CGO_CFLAGS)' $(GO) test -tags=$(subst $(space),$(comma),$(BUILD_GO_TAGS)) $(GO_TEST_RACE) -ldflags='$(LD_FLAGS)' `$(GO) list -tags=$(subst $(space),$(comma),$(BUILD_GO_TAGS)) $(GO_CHECK_UNIT_DIRS)`

.PHONY: clean-gocache
clean-gocache:
	-chmod -R u+w -- '$(K0S_GO_BUILD_CACHE)/go/mod'
	rm -rf -- '$(K0S_GO_BUILD_CACHE)/go'

.PHONY: clean-docker-image
clean-docker-image: IID_FILES = .k0sbuild.docker-image.k0s
clean-docker-image:
	$(clean-iid-files)

.PHONY: clean-airgap-image-bundles
clean-airgap-image-bundles: IID_FILES = .k0sbuild.image-bundler.stamp
clean-airgap-image-bundles:
	$(clean-iid-files)
	-rm airgap-images.txt
	-rm airgap-image-bundle-linux-amd64.tar airgap-image-bundle-linux-arm64.tar airgap-image-bundle-linux-arm.tar

.PHONY: clean
clean: clean-gocache clean-docker-image clean-airgap-image-bundles
	-rm -f pkg/assets/zz_generated_offsets_*.go k0s k0s.exe .bins.*stamp bindata* static/zz_generated_assets.go
	-rm -rf $(K0S_GO_BUILD_CACHE)
	-find pkg/apis -type f \( -name .client-gen.stamp -or -name .controller-gen.stamp \) -delete
	-rm -f hack/.copyright.stamp
	-$(MAKE) -C docs clean
	-$(MAKE) -C embedded-bins clean
	-$(MAKE) -C inttest clean

.PHONY: docs
docs:
	$(MAKE) -C docs

.PHONY: docs-serve-dev
docs-serve-dev: DOCS_DEV_PORT ?= 8000
docs-serve-dev:
	$(MAKE) -C docs .docker-image.serve-dev.stamp
	docker run --rm \
	  -e KUBERNETES_VERSION='$(kubernetes_version)' \
	  -v "$(CURDIR):/k0s:ro" \
	  -p '$(DOCS_DEV_PORT):8000' \
	  k0sdocs.docker-image.serve-dev

sbom/spdx.json: go.mod
	mkdir -p -- '$(dir $@)'
	docker run --rm \
	  -v "$(CURDIR)/go.mod:/k0s/go.mod" \
	  -v "$(CURDIR)/embedded-bins/staging/linux/bin:/k0s/bin" \
	  -v "$(CURDIR)/syft.yaml:/tmp/syft.yaml" \
	  -v "$(CURDIR)/sbom:/out" \
	  --user $(BUILD_UID):$(BUILD_GID) \
	  anchore/syft:v0.90.0 \
	  /k0s -o spdx-json@2.2=/out/spdx.json -c /tmp/syft.yaml

.PHONY: sign-sbom
sign-sbom: sbom/spdx.json
	docker run --rm \
	  -v "$(CURDIR):/k0s" \
	  -v "$(CURDIR)/sbom:/out" \
	  -e COSIGN_PASSWORD="$(COSIGN_PASSWORD)" \
	  gcr.io/projectsigstore/cosign:v2.2.3 \
	  sign-blob \
	  --key /k0s/cosign.key \
	  --tlog-upload=false \
	  /k0s/sbom/spdx.json --output-file /out/spdx.json.sig

.PHONY: sign-pub-key
sign-pub-key:
	docker run --rm \
	  -v "$(CURDIR):/k0s" \
	  -v "$(CURDIR)/sbom:/out" \
	  -e COSIGN_PASSWORD="$(COSIGN_PASSWORD)" \
	  gcr.io/projectsigstore/cosign:v2.2.3 \
	  public-key \
	  --key /k0s/cosign.key --output-file /out/cosign.pub

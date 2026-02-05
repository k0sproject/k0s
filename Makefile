include embedded-bins/Makefile.variables
include inttest/Makefile.variables
include hack/tools/Makefile.variables

ifndef HOST_ARCH
HOST_HARDWARE := $(shell uname -m)
ifneq (, $(filter $(HOST_HARDWARE), aarch64 arm64))
  HOST_ARCH := arm64
else ifneq (, $(filter $(HOST_HARDWARE), armv8l armv7l arm))
  HOST_ARCH := arm
else ifneq (, $(filter $(HOST_HARDWARE), riscv64))
  HOST_ARCH := riscv64
else
  ifeq (, $(filter $(HOST_HARDWARE), x86_64 amd64 x64))
    $(warning unknown machine hardware name $(HOST_HARDWARE), assuming amd64)
  endif
  HOST_ARCH := amd64
endif
endif

ifeq ($(OS),Windows_NT)
CYGPATH := $(shell cygpath --version >/dev/null 2>&1 && echo cygpath)
cygpath = $(if $(CYGPATH),$(shell '$(CYGPATH)' -- '$(1)'),$(1))
PATHSEP := $(if $(CYGPATH),:,;)
else
cygpath = $(1)
PATHSEP := :
endif

FIND ?= find
DOCKER ?= docker

K0S_GO_BUILD_CACHE ?= build/cache

GO_SRCS := $(shell $(FIND) . -type f -name "*.go" -not -path "./$(K0S_GO_BUILD_CACHE)/*" -not -path "./inttest/*" -not -name "*_test.go" -not -name "zz_generated*")
GO_CHECK_UNIT_DIRS := . ./cmd/... ./pkg/... ./internal/... ./static/... ./hack/...

# Disable Docker build integration if DOCKER is set to the empty string.
ifeq ($(DOCKER),)
  GO_ENV_REQUISITES ?=
  GO_ENV ?= PATH='$(call cygpath,$(abspath $(K0S_GO_BUILD_CACHE))/go/bin)$(PATHSEP)'"$$PATH" \
    GOBIN="$(abspath $(K0S_GO_BUILD_CACHE))/go/bin" \
    GOCACHE="$(abspath $(K0S_GO_BUILD_CACHE))/go/build" \
    GOMODCACHE="$(abspath $(K0S_GO_BUILD_CACHE))/go/mod"
  GO ?= $(GO_ENV) go
else
  GO_ENV_REQUISITES ?= .k0sbuild.docker-image.k0s
  GO_ENV ?= $(DOCKER) run --rm \
    -v '$(realpath $(K0S_GO_BUILD_CACHE))':/run/k0s-build \
    -v '$(CURDIR)':/go/src/github.com/k0sproject/k0s \
    -w /go/src/github.com/k0sproject/k0s \
    -e GOOS \
    -e CGO_ENABLED \
    -e CGO_CFLAGS \
    -e GOARCH \
    -e XDG_CACHE_HOME=/run/k0s-build \
    --user $(BUILD_UID):$(BUILD_GID) \
    $(DOCKER_RUN_OPTS) -- '$(shell cat .k0sbuild.docker-image.k0s)'
  GO ?= $(GO_ENV) go
endif

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   none	does not embed any binaries
EMBEDDED_BINS_BUILDMODE ?= docker

# k0s runs on linux even if it's built on mac or windows
TARGET_OS ?= linux
BUILD_UID ?= $(shell id -u)
BUILD_GID ?= $(shell id -g)
BUILD_GO_TAGS ?= osusergo
BUILD_GO_FLAGS = -tags=$(subst $(space),$(comma),$(BUILD_GO_TAGS)) -buildvcs=false
BUILD_CGO_CFLAGS :=
BUILD_GO_LDFLAGS_EXTRA :=
DEBUG ?= false

VERSION ?= $(shell git describe --tags 2>/dev/null || printf v%s-dev+k0s '$(kubernetes_version)')
ifeq ($(DEBUG), false)
BUILD_GO_FLAGS += -trimpath
LD_FLAGS ?= -w -s
endif

# https://reproducible-builds.org/docs/source-date-epoch/#makefile
# https://reproducible-builds.org/docs/source-date-epoch/#git
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct 2>/dev/null || date -u +%s)
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
LD_FLAGS += -X k8s.io/component-base/version.gitMajor=$(word 1,$(subst ., ,$(kubernetes_version)))
LD_FLAGS += -X k8s.io/component-base/version.gitMinor=$(word 2,$(subst ., ,$(kubernetes_version)))
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
	$(DOCKER) build --progress=plain --iidfile '$@' \
	  --build-arg BUILDKIT_DOCKERFILE_CHECK=skip=InvalidDefaultArgInFrom \
	  --build-arg BUILDIMAGE=$(GOLANG_IMAGE) \
	  -t k0sbuild.docker-image.k0s - <build/Dockerfile

go.sum: go.mod $(GO_ENV_REQUISITES)
	$(GO) mod tidy && touch -c -- '$@'

# List of all the custom APIs that k0s defines.
api_group_versions := $(foreach path,$(wildcard pkg/apis/*/v*/doc.go),$(path:pkg/apis/%/doc.go=%))

# Declare the requisites for the generators operating on API group versions.
api_group_version_targets := .controller-gen.stamp zz_generated.register.go
$(foreach gv,$(api_group_versions),$(eval $(foreach t,$(api_group_version_targets),pkg/apis/$(gv)/$(t)): $$(shell $(FIND) pkg/apis/$(gv)/ -maxdepth 1 -type f -name "*.go" -not -name "*_test.go" -not -name "zz_generated*")))

# Run controller-gen for each API group version.
controller_gen_targets := $(foreach gv,$(api_group_versions),pkg/apis/$(gv)/.controller-gen.stamp)
codegen_targets := $(controller_gen_targets)
$(controller_gen_targets): $(GO_ENV_REQUISITES) hack/tools/boilerplate.go.txt hack/tools/Makefile.variables
	rm -rf 'static/_crds/$(dir $(@:pkg/apis/%/.controller-gen.stamp=%))'
	gendir="$$(mktemp -d .controller-gen.tmp.XXXXXX)" \
	  && trap "rm -rf -- $$gendir" INT EXIT \
	  && CGO_ENABLED=0 $(GO) run sigs.k8s.io/controller-tools/cmd/controller-gen@v$(controller-tools_version) \
	    paths="./$(dir $@)..." \
	    object:headerFile=hack/tools/boilerplate.go.txt output:object:dir="$$gendir" \
	    crd output:crd:dir='static/_crds/$(dir $(@:pkg/apis/%/.controller-gen.stamp=%))' \
	  && mv -f -- "$$gendir"/zz_generated.deepcopy.go '$(dir $@).'
	touch -- '$@'

# Run register-gen for each API group version.
register_gen_targets := $(foreach gv,$(api_group_versions),pkg/apis/$(gv)/zz_generated.register.go)
codegen_targets += $(register_gen_targets)
$(register_gen_targets): $(GO_ENV_REQUISITES) hack/tools/boilerplate.go.txt embedded-bins/Makefile.variables
	CGO_ENABLED=0 $(GO) run k8s.io/code-generator/cmd/register-gen@v$(kubernetes_version:1.%=0.%) \
	  --go-header-file=hack/tools/boilerplate.go.txt \
	  --output-file='_$(notdir $@).tmp' \
	  'github.com/k0sproject/k0s/$(dir $@)' || { \
	    ret=$$?; \
	    rm -f -- '$(dir $@)_$(notdir $@).tmp'; \
	    exit $$ret; \
	  }
	mv -- '$(dir $@)_$(notdir $@).tmp' '$@'

# Generate the k0s client-go clientset based on all custom API group versions.
clientset_input_dirs := $(foreach gv,$(api_group_versions),pkg/apis/$(gv))
codegen_targets += pkg/client/clientset/.client-gen.stamp
pkg/client/clientset/.client-gen.stamp: $(shell $(FIND) $(clientset_input_dirs) -type f -name "*.go" -not -name "*_test.go" -not -name "zz_generated*")
pkg/client/clientset/.client-gen.stamp: $(GO_ENV_REQUISITES) hack/tools/boilerplate.go.txt embedded-bins/Makefile.variables
	gendir="$$(mktemp -d .client-gen.tmp.XXXXXX)" \
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

embedded-binaries-linux.zip: .bins.linux.stamp
embedded-binaries-windows.zip: .bins.windows.stamp
embedded-binaries-linux.zip embedded-binaries-windows.zip: $(GO_ENV_REQUISITES) go.sum hack/zip-files/* embedded-bins/Makefile.variables
	CGO_ENABLED=0 $(GO) run -tags=hack hack/zip-files/main.go embedded-bins/staging/$(@:embedded-binaries-%.zip=%)/bin/* >$@

# gen-bindata produces the compressed bindata file and the corresponding Go file
# containing the offsets. Ideally, these would be declared as grouped targets,
# but that feature is too recent for the targeted GNU Make 3.81 compatibility.
# As a workaround, make the offsets depend on the bindata and include the
# gen-bindata invocation in the bindata.
bindata_linux: .bins.linux.stamp
bindata_windows: .bins.windows.stamp
pkg/assets/zz_generated_offsets_linux.go: bindata_linux
pkg/assets/zz_generated_offsets_windows.go: bindata_windows
bindata_linux bindata_windows: $(GO_ENV_REQUISITES) go.sum hack/gen-bindata/* hack/gen-bindata/cmd/*
	GOOS=${GOHOSTOS} $(GO) run -tags=hack hack/gen-bindata/cmd/main.go -o bindata_$(@:bindata_%=%) -pkg assets \
	  -gofile pkg/assets/zz_generated_offsets_$(@:bindata_%=%).go \
	  -prefix embedded-bins/staging/$(@:bindata_%=%)/ embedded-bins/staging/$(@:bindata_%=%)/bin

ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
BUILD_GO_TAGS += noembedbins
else
k0s.bare: pkg/assets/zz_generated_offsets_linux.go
k0s.bare.exe: pkg/assets/zz_generated_offsets_windows.go
k0s: bindata_linux
k0s.exe: bindata_windows
endif

k0s k0s.bare: TARGET_OS = linux
k0s.bare: BUILD_GO_CGO_ENABLED = 1

k0s.exe k0s.bare.exe: TARGET_OS = windows
k0s.bare.exe: BUILD_GO_CGO_ENABLED = 0

.INTERMEDIATE: k0s.bare k0s.bare.exe
k0s.bare.exe k0s.bare: $(GO_ENV_REQUISITES) go.sum $(codegen_targets) $(GO_SRCS) $(shell find static/manifests/calico static/manifests/windows -type f)
	CGO_ENABLED=$(BUILD_GO_CGO_ENABLED) CGO_CFLAGS='$(BUILD_CGO_CFLAGS)' GOOS=$(TARGET_OS) $(GO) build $(BUILD_GO_FLAGS) -ldflags='$(LD_FLAGS)' -o '$@' main.go

k0s: k0s.bare
k0s.exe: k0s.bare.exe

k0s.exe k0s:
	mv $(@:k0s%=k0s.bare%) $@
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
bindata:
ifneq ($(EMBEDDED_BINS_BUILDMODE),none)
bindata: pkg/assets/zz_generated_offsets_$(TARGET_OS).go
endif

.PHONY: lint-copyright
lint-copyright:
	hack/copyright.sh

.PHONY: lint-go
lint-go: GOLANGCI_LINT_FLAGS ?=
lint-go: $(GO_ENV_REQUISITES) go.sum bindata
	CGO_ENABLED=0 $(GO) install github.com/golangci/golangci-lint/v$(word 1,$(subst ., ,$(golangci-lint_version)))/cmd/golangci-lint@v$(golangci-lint_version)
	GOLANGCI_LINT_CACHE='$(abspath $(K0S_GO_BUILD_CACHE))/golangci-lint' GO_CFLAGS='$(BUILD_CGO_CFLAGS)' $(GO_ENV) golangci-lint run --verbose --build-tags=$(subst $(space),$(comma),$(BUILD_GO_TAGS)) $(GOLANGCI_LINT_FLAGS) $(GO_LINT_DIRS)

.PHONY: lint
lint: lint-copyright lint-go

airgap-images.txt: k0s $(GO_ENV_REQUISITES)
	$(GO_ENV) ./k0s airgap list-images --all > '$@'

airgap-image-bundle-linux-amd64.tar:   TARGET_PLATFORM := linux/amd64
airgap-image-bundle-linux-arm64.tar:   TARGET_PLATFORM := linux/arm64
airgap-image-bundle-linux-arm.tar:     TARGET_PLATFORM := linux/arm/v7
airgap-image-bundle-linux-riscv64.tar: TARGET_PLATFORM := linux/riscv64
airgap-image-bundle-linux-amd64.tar \
airgap-image-bundle-linux-arm64.tar \
airgap-image-bundle-linux-arm.tar \
airgap-image-bundle-linux-riscv64.tar: k0s airgap-images.txt
	set -- $$(cat airgap-images.txt) && \
	$(GO_ENV) ./k0s airgap bundle-artifacts --concurrency=1 -v --platform='$(TARGET_PLATFORM)' -o '$@' "$$@"

ipv6-test-images.txt: $(GO_ENV_REQUISITES) embedded-bins/Makefile.variables hack/gen-test-images-list/main.go
	{ \
	  echo "docker.io/library/nginx:1.29.4-alpine"; \
	  echo "docker.io/curlimages/curl:8.18.0"; \
	  echo "docker.io/library/alpine:$(alpine_version)"; \
	  echo "docker.io/sonobuoy/sonobuoy:v$(sonobuoy_version)"; \
	  echo "registry.k8s.io/conformance:v$(kubernetes_version)"; \
	  $(GO) run -tags=hack ./hack/gen-test-images-list; \
	} >'$@'

ipv6-test-image-bundle-linux-amd64.tar:   TARGET_PLATFORM := linux/amd64
ipv6-test-image-bundle-linux-arm64.tar:   TARGET_PLATFORM := linux/arm64
ipv6-test-image-bundle-linux-arm.tar:     TARGET_PLATFORM := linux/arm/v7
ipv6-test-image-bundle-linux-riscv64.tar: TARGET_PLATFORM := linux/riscv64
ipv6-test-image-bundle-linux-amd64.tar \
ipv6-test-image-bundle-linux-arm64.tar \
ipv6-test-image-bundle-linux-arm.tar \
ipv6-test-image-bundle-linux-riscv64.tar: k0s ipv6-test-images.txt
	set -- $$(cat ipv6-test-images.txt) && \
	$(GO_ENV) ./k0s airgap bundle-artifacts -v --platform='$(TARGET_PLATFORM)' -o '$@' "$$@"

.PHONY: $(smoketests)
$(air_gapped_smoketests) $(ipv6_smoketests): airgap-image-bundle-linux-$(HOST_ARCH).tar
$(ipv6_smoketests): ipv6-test-image-bundle-linux-$(HOST_ARCH).tar
$(smoketests): k0s
	$(MAKE) -C inttest \
		K0S_IMAGES_BUNDLE='$(CURDIR)/airgap-image-bundle-linux-$(HOST_ARCH).tar' \
		K0S_EXTRA_IMAGES_BUNDLE='$(CURDIR)/ipv6-test-image-bundle-linux-$(HOST_ARCH).tar' \
		$@

.PHONY: smoketests
smoketests: $(smoketests)

.PHONY: check-unit
ifneq (, $(filter $(HOST_ARCH), arm riscv64))
check-unit: GO_TEST_RACE ?=
else
check-unit: GO_TEST_RACE ?= -race
endif
check-unit: BUILD_GO_TAGS += hack
check-unit: $(GO_ENV_REQUISITES) go.sum bindata
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
clean-airgap-image-bundles:
	-rm airgap-images.txt
	-rm airgap-image-bundle-linux-amd64.tar airgap-image-bundle-linux-arm64.tar airgap-image-bundle-linux-arm.tar  airgap-image-bundle-linux-riscv64.tar

.PHONY: clean
clean: clean-gocache clean-docker-image clean-airgap-image-bundles
	-rm -f pkg/assets/zz_generated_offsets_*.go k0s k0s.exe k0s.bare k0s.bare.exe .bins.*stamp bindata*
	-rm -f embedded-binaries-linux.zip embedded-binaries-windows.zip
	-rm -rf $(K0S_GO_BUILD_CACHE)
	-find pkg/apis -type f -name .controller-gen.stamp -delete
	-rm pkg/client/clientset/.client-gen.stamp
	-rm -f hack/.copyright.stamp
	-rm -f spdx.json
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
	$(DOCKER) run --rm \
	  -e PYTHONPATH=/k0s/docs/mkdocs_modules \
	  -e K0S_VERSION=$(VERSION) \
	  -v "$(CURDIR):/k0s:ro" \
	  -p '$(DOCS_DEV_PORT):8000' \
	  $(DOCKER_RUN_OPTS) k0sdocs.docker-image.serve-dev

spdx.json: syft.yaml go.mod .bins.$(TARGET_OS).stamp
	$(DOCKER) run --rm \
	  -v '$(CURDIR)/syft.yaml':/k0s/syft.yaml:ro \
	  -v '$(CURDIR)/go.mod':/k0s/go.mod:ro \
	  -v '$(CURDIR)/embedded-bins/staging/$(TARGET_OS)/bin':/k0s/bin:ro \
	  -w /k0s \
	  $(DOCKER_RUN_OPTS) docker.io/anchore/syft:v1.41.1 \
	  --source-name k0s --source-version '$(VERSION)' \
	  -c syft.yaml -o spdx-json@2.2 . >'$@'

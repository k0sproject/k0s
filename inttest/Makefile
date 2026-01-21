ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)
DOCKER ?= docker

sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v${sonobuoy_version}/sonobuoy_${sonobuoy_version}_$(OS)_$(ARCH).tar.gz

curl = curl --proto '=https' --tlsv1.2 -sSL

bins = bin/sonobuoy

include ../embedded-bins/Makefile.variables
include Makefile.variables

.PHONY: all
all: $(bins) .bootloose-alpine.stamp

bin:
	mkdir -p $@

bin/sonobuoy: | bin
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

bootloose_alpine_build_cmdline := \
	--build-arg GOLANG_IMAGE=$(golang_buildimage) \
	--build-arg ALPINE_VERSION=$(alpine_patch_version) \
	--build-arg ETCD_VERSION=$(etcd_version) \
	--build-arg HELM_VERSION=$(helm_version) \
	-t bootloose-alpine \
	-f bootloose-alpine/Dockerfile \
	bootloose-alpine

.bootloose-alpine.stamp: $(shell find bootloose-alpine -type f)
	$(DOCKER) build --progress=plain --build-arg TARGETARCH=$(ARCH) $(bootloose_alpine_build_cmdline)
	touch $@

# This is a special target to test the bootloose alpine image locally for all supported platforms.
.PHONY: check-bootloose-alpine-buildx
check-bootloose-alpine-buildx:
	$(DOCKER) buildx build --progress=plain --platform linux/amd64,linux/arm64,linux/arm/v7 $(bootloose_alpine_build_cmdline)

.bootloose-k0s.stamp: K0S_PATH ?= $(realpath ../k0s)
.bootloose-k0s.stamp: .bootloose-alpine.stamp
	$(DOCKER) build --progress=plain \
	  --build-arg K0S_PATH=$(notdir $(K0S_PATH)) \
	  -t bootloose-k0s \
	  -f bootloose-k0s/Dockerfile \
	  $(dir $(K0S_PATH))
	touch $@

.update-server.stamp: .bootloose-alpine.stamp update-server/Dockerfile $(wildcard update-server/html/**/*.html)
	$(DOCKER) build --progress=plain -t update-server --build-arg BASE=bootloose-alpine -f update-server/Dockerfile update-server
	touch $@

check-network: bin/sonobuoy
	$(realpath bin/sonobuoy) run --wait=1200 --plugin=e2e --plugin-env=e2e.E2E_USE_GO_RUNNER=true \
		--e2e-focus='\[sig-network\].*\[Conformance\]' \
		--e2e-skip='\[Serial\]' --e2e-parallel=y \
		--kubernetes-version=v$(kubernetes_version)

check-conformance: bin/sonobuoy
	$(realpath bin/sonobuoy) run --wait=1200 \
		--mode=certified-conformance \
		--plugin-env=e2e.E2E_EXTRA_ARGS="--ginkgo.v" \
		--kubernetes-version=v$(kubernetes_version)

get-conformance-results: bin/sonobuoy
	$(realpath bin/sonobuoy) retrieve

TIMEOUT ?= 6m

# Autopilot 3x3 HA test can take a while to run
check-ap-ha3x3: export K0S_UPDATE_TO_VERSION ?= $(shell ../k0s version)

check-ap-updater: .update-server.stamp
check-ap-updater-periodic: .update-server.stamp
check-ap-updater-periodic: TIMEOUT=10m

# Backup check runs two scenarios
check-backup: TIMEOUT=10m

# Config change smoke runs actually many cases hence a bit longer timeout
check-configchange: TIMEOUT=8m

check-ctr: TIMEOUT=10m

check-kubeletcertrotate: TIMEOUT=15m

check-network-conformance-calico: TIMEOUT=15m
check-network-conformance-calico-nft: TIMEOUT=15m
check-network-conformance-kuberouter: TIMEOUT=15m
check-network-conformance-kuberouter-nft: TIMEOUT=15m

check-nllb: TIMEOUT=15m
check-nllb-ipv6: TIMEOUT=15m

.PHONY: $(smoketests)

# Determine the inttest package automatically by finding the longest prefix of
# the target name for which a directory exists.
determine_inttest_package = package='$(subst check-,,$@)'; prefix=''; \
  for part in $(subst -, ,$(@:check-%=%)); do \
    { [ -z "$$prefix" ] && prefix="$$part"; } || prefix="$$prefix-$$part"; \
    [ ! -d "$$prefix" ] || package="$$prefix"; \
  done; \
  echo "$$package"

$(smoketests): export K0S_IMAGES_BUNDLE ?= $(realpath ../airgap-image-bundle-linux-$(ARCH).tar)
# K0S_EXTRA_IMAGES_BUNDLE is used for the IPv6 test images
$(smoketests): export K0S_EXTRA_IMAGES_BUNDLE ?= $(realpath ../ipv6-test-image-bundle-linux-$(ARCH).tar)
$(smoketests): K0S_INTTEST_PACKAGE ?= $(shell $(determine_inttest_package))
$(smoketests): .bootloose-alpine.stamp
	K0S_INTTEST_TARGET=$(subst check-,,$@) go test -count=1 -v -timeout $(TIMEOUT) github.com/k0sproject/k0s/inttest/$(K0S_INTTEST_PACKAGE)
.PHONY: clean

clean:
	-[ -x bin/sonobuoy ] && bin/sonobuoy delete
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp

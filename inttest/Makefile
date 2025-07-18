ARCH := $(shell go env GOARCH)
OS := $(shell go env GOOS)

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
	docker build --progress=plain --build-arg TARGETARCH=$(ARCH) $(bootloose_alpine_build_cmdline)
	touch $@

# This is a special target to test the bootloose alpine image locally for all supported platforms.
.PHONY: check-bootloose-alpine-buildx
check-bootloose-alpine-buildx:
	docker buildx build --progress=plain --platform linux/amd64,linux/arm64,linux/arm/v7 $(bootloose_alpine_build_cmdline)

.bootloose-k0s.stamp: K0S_PATH ?= $(realpath ../k0s)
.bootloose-k0s.stamp: .bootloose-alpine.stamp
	docker build --progress=plain \
	  --build-arg K0S_PATH=$(notdir $(K0S_PATH)) \
	  -t bootloose-k0s \
	  -f bootloose-k0s/Dockerfile \
	  $(dir $(K0S_PATH))
	touch $@

.update-server.stamp: .bootloose-alpine.stamp update-server/Dockerfile $(wildcard update-server/html/**/*.html)
	docker build --progress=plain -t update-server --build-arg BASE=bootloose-alpine -f update-server/Dockerfile update-server
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

check-ctr: TIMEOUT=10m

# Config change smoke runs actually many cases hence a bit longer timeout
check-configchange: TIMEOUT=8m

# Backup check runs two scenarios
check-backup: TIMEOUT=10m

# Autopilot 3x3 HA test can take a while to run
check-ap-ha3x3: K0S_UPDATE_FROM_BIN ?= ../k0s
check-ap-ha3x3: export K0S_UPDATE_FROM_PATH ?= $(realpath $(K0S_UPDATE_FROM_BIN))
check-ap-ha3x3: export K0S_UPDATE_TO_VERSION ?= $(shell ../k0s version)

check-ap-controllerworker: K0S_UPDATE_FROM_BIN ?= ../k0s
check-ap-controllerworker: export K0S_UPDATE_FROM_PATH ?= $(realpath $(K0S_UPDATE_FROM_BIN))

check-customports-dynamicconfig: export K0S_ENABLE_DYNAMIC_CONFIG=true

check-kubeletcertrotate: TIMEOUT=15m

check-cplb-userspace-extaddr: export K0S_USE_EXTERNAL_ADDRESS=yes

check-dualstack-calico: export K0S_NETWORK=calico

check-dualstack-calico-dynamicconfig: export K0S_ENABLE_DYNAMIC_CONFIG=true
check-dualstack-calico-dynamicconfig: export K0S_NETWORK=calico

check-dualstack-kuberouter: export K0S_NETWORK=kube-router

check-dualstack-kuberouter-dynamicconfig: export K0S_ENABLE_DYNAMIC_CONFIG=true
check-dualstack-kuberouter-dynamicconfig: export K0S_NETWORK=kuberouter

check-ap-updater: .update-server.stamp
check-ap-updater-periodic: .update-server.stamp
check-ap-updater-periodic: TIMEOUT=10m

check-network-conformance-kuberouter: TIMEOUT=15m
check-network-conformance-kuberouter: export K0S_NETWORK_CONFORMANCE_CNI=kuberouter
check-network-conformance-calico: TIMEOUT=15m
check-network-conformance-calico: export K0S_NETWORK_CONFORMANCE_CNI=calico

check-network-conformance-kuberouter-nft: TIMEOUT=15m
check-network-conformance-kuberouter-nft: export K0S_NETWORK_CONFORMANCE_CNI=kuberouter
check-network-conformance-kuberouter-nft: export K0S_NETWORK_CONFORMANCE_PROXY_MODE=nftables
check-network-conformance-calico-nft: TIMEOUT=15m
check-network-conformance-calico-nft: export K0S_NETWORK_CONFORMANCE_CNI=calico
check-network-conformance-calico-nft: export K0S_NETWORK_CONFORMANCE_PROXY_MODE=nftables

check-metricsscraper-singlenode: export K0S_SINGLENODE=1

check-nllb: TIMEOUT=15m

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
$(smoketests): K0S_INTTEST_PACKAGE ?= $(shell $(determine_inttest_package))
$(smoketests): .bootloose-alpine.stamp
	K0S_INTTEST_TARGET=$(subst check-,,$@) go test -count=1 -v -timeout $(TIMEOUT) github.com/k0sproject/k0s/inttest/$(K0S_INTTEST_PACKAGE)
.PHONY: clean

clean:
	-[ -x bin/sonobuoy ] && bin/sonobuoy delete
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp

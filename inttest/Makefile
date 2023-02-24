
ARCH = $(shell go env GOARCH)
OS = $(shell go env GOOS)
K0S_UPDATE_TO_VERSION ?= $(shell ../k0s version)

SONOBUOY_VERSION ?= 0.56.11
sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_$(OS)_$(ARCH).tar.gz

curl = curl -L --silent

bins = bin/sonobuoy

include ../embedded-bins/Makefile.variables

ifeq ($(ARCH),amd64)
etcd_arch = amd64
else
etcd_arch = arm64
endif

.PHONY: all
all: $(bins) .footloose-alpine.stamp

bin:
	mkdir -p $@

bin/sonobuoy: | bin
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

.footloose-alpine.stamp: footloose-alpine/Dockerfile $(shell find footloose-alpine/root -type f)
	docker build \
	  --build-arg ALPINE_VERSION=$(alpine_patch_version) \
	  --build-arg ETCD_ARCH=$(etcd_arch) \
	  --build-arg ETCD_VERSION=$(etcd_version) \
	  --build-arg HELM_VERSION=$(helm_version) \
	  --build-arg KUBE_VERSION=$(kubernetes_version) \
	  -t footloose-alpine \
	  -f footloose-alpine/Dockerfile \
	  footloose-alpine
	touch $@

.update-server.stamp: .footloose-alpine.stamp update-server/Dockerfile update-server/html/stable/index.yaml
	docker build -t update-server --build-arg BASE=footloose-alpine -f update-server/Dockerfile update-server
	touch $@

check-network: bin/sonobuoy
	$(realpath bin/sonobuoy) run --wait=1200 --plugin=e2e --plugin-env=e2e.E2E_USE_GO_RUNNER=true \
		--e2e-focus='\[sig-network\].*\[Conformance\]' \
		--e2e-skip='\[Serial\]' --e2e-parallel=y \
		--kubernetes-version=v$(kubernetes_version)

check-conformance: bin/sonobuoy
	$(realpath bin/sonobuoy) run --wait=1200 \
		--mode=certified-conformance \
		--kubernetes-version=v$(kubernetes_version)

get-conformance-results: bin/sonobuoy
	$(realpath bin/sonobuoy) retrieve

TIMEOUT ?= 4m

check-ctr: TIMEOUT=10m
check-byocri: TIMEOUT=5m
# readiness check for metric tests takes between around 5 and 6 minutes.
check-metrics: TIMEOUT=6m
check-metricsscraper: TIMEOUT=6m

check-calico: TIMEOUT=6m

# Establishing konnectivity tunnels with the LB in place takes a while, thus a bit longer timeout for the smoke
check-customports: TIMEOUT=6m

# Config change smoke runs actually many cases hence a bit longer timeout
check-configchange: TIMEOUT=8m

# Backup check runs two scenarios
check-backup: TIMEOUT=10m

# Autopilot 3x3 HA test can take a while to run
check-ap-ha3x3: K0S_UPDATE_FROM_BIN ?= ../k0s
check-ap-ha3x3: K0S_UPDATE_FROM_PATH ?= $(realpath $(K0S_UPDATE_FROM_BIN))
check-ap-ha3x3: TIMEOUT=6m

check-customports-dynamicconfig: export K0S_ENABLE_DYNAMIC_CONFIG=true
check-customports-dynamicconfig: TEST_PACKAGE=customports

check-kubeletcertrotate: TIMEOUT=15m

check-dualstack-dynamicconfig: export K0S_ENABLE_DYNAMIC_CONFIG=true
check-dualstack-dynamicconfig: TEST_PACKAGE=dualstack

check-ap-updater: .update-server.stamp

check-network-conformance-kuberouter: TIMEOUT=15m
check-network-conformance-kuberouter: export K0S_NETWORK_CONFORMANCE_CNI=kuberouter
check-network-conformance-kuberouter: TEST_PACKAGE=network-conformance
check-network-conformance-calico: TIMEOUT=15m
check-network-conformance-calico: export K0S_NETWORK_CONFORMANCE_CNI=calico
check-network-conformance-calico: TEST_PACKAGE=network-conformance

check-nllb: TIMEOUT=10m

.PHONY: $(smoketests)
include Makefile.variables

$(smoketests): K0S_PATH ?= $(realpath ../k0s)
$(smoketests): K0S_IMAGES_BUNDLE ?= $(realpath ../airgap-image-bundle-linux-amd64.tar)
$(smoketests): .footloose-alpine.stamp
$(smoketests): TEST_PACKAGE ?= $(subst check-,,$@)
$(smoketests):
	K0S_PATH='$(K0S_PATH)' \
	K0S_UPDATE_FROM_PATH='$(K0S_UPDATE_FROM_PATH)' \
	K0S_IMAGES_BUNDLE='$(K0S_IMAGES_BUNDLE)' \
	K0S_UPDATE_TO_VERSION='$(K0S_UPDATE_TO_VERSION)' \
	go test -count=1 -v -timeout $(TIMEOUT) github.com/k0sproject/k0s/inttest/$(TEST_PACKAGE)
.PHONY: clean

clean:
	-[ -x bin/sonobuoy ] && bin/sonobuoy delete
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp

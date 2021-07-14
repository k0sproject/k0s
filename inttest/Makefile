
ARCH = $(shell go env GOARCH)
OS = $(shell go env GOOS)

sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.20.0/sonobuoy_0.20.0_$(OS)_$(ARCH).tar.gz

curl = curl -L --silent

bins = bin/sonobuoy

.PHONY: all
all: $(bins) .footloose-alpine.stamp

bin:
	mkdir -p $@

bin/sonobuoy: | bin
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

.footloose-alpine.stamp: footloose-alpine/Dockerfile
	docker build -t footloose-alpine -f $< $(dir $<)
	touch $@

check-network: bin/sonobuoy .footloose-alpine.stamp
	K0S_PATH=$(realpath ../k0s) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 20m github.com/k0sproject/k0s/inttest/sonobuoy/ -run ^TestFootlooseNetworkSuite$

check-network-vm: bin/sonobuoy
	K0S_PATH=$(realpath ../k0s) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 30m github.com/k0sproject/k0s/inttest/sonobuoy/ -run ^TestVMNetworkSuite$

TIMEOUT ?= 4m

check-byocri: TIMEOUT=5m
# readiness check for metric tests takes between around 5 and 6 minutes.
check-metrics: TIMEOUT=6m

# Establishing konnectivity tunnels with the LB in place takes a while, thus a bit longer timeout for the smoke
check-customports: TIMEOUT=6m

.PHONY: $(smoketests)
include Makefile.variables

$(smoketests): .footloose-alpine.stamp
	K0S_IMAGES_BUNDLE="$(realpath ../image-bundle/bundle.tar)" K0S_PATH="$(realpath ../k0s)" go test -count=1 -v -timeout $(TIMEOUT) github.com/k0sproject/k0s/inttest/$(subst check-,,$@)

.PHONY: clean
clean:
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp

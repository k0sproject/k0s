
ARCH = $(shell go env GOARCH)
OS = $(shell go env GOOS)
K0S_PATH := $(realpath ../k0s)

sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_$(OS)_$(ARCH).tar.gz

curl = curl -L --silent

bins = bin/sonobuoy

.PHONY: all
all: $(bins) .footloose-alpine.stamp

bin:
	mkdir -p $@

bin/sonobuoy: | bin
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

kine.conf:
	printf "spec:\n  storage:\n    type: kine\n" > $@

.footloose-alpine.stamp: footloose-alpine/Dockerfile
	docker build -t footloose-alpine -f $< $(dir $<)

check-network: bin/sonobuoy .footloose-alpine.stamp
	K0S_PATH=$(K0S_PATH) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 20m github.com/k0sproject/k0s/inttest/sonobuoy/ -run ^TestFootlooseNetworkSuite$

check-network-vm: bin/sonobuoy
	K0S_PATH=$(K0S_PATH) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 20m github.com/k0sproject/k0s/inttest/sonobuoy/ -run ^TestVMNetworkSuite$

check-basic: .footloose-alpine.stamp
	K0S_PATH=$(K0S_PATH) go test -count=1 -v -timeout 5m github.com/k0sproject/k0s/inttest/basic

check-hacontrolplane: .footloose-alpine.stamp
	K0S_PATH=$(K0S_PATH) go test -count=1 -v -timeout 3m github.com/k0sproject/k0s/inttest/hacontrolplane

check-kine: .footloose-alpine.stamp kine.conf
	K0S_PATH=$(K0S_PATH) K0S_CONFIG=$(CURDIR)/kine.conf \
		 go test -count=1 -v -timeout 5m github.com/k0sproject/k0s/inttest/basic

.PHONY: clean
clean:
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp kine.conf

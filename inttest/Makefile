
ARCH = $(shell go env GOARCH)
OS = $(shell go env GOOS)

sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_$(OS)_$(ARCH).tar.gz

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

check-network: bin/sonobuoy .footloose-alpine.stamp
	MKE_PATH=$(realpath ../mke) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 20m github.com/Mirantis/mke/inttest/sonobuoy/ -run ^TestFootlooseNetworkSuite$

check-network-vm: bin/sonobuoy
	MKE_PATH=$(realpath ../mke) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 20m github.com/Mirantis/mke/inttest/sonobuoy/ -run ^TestVMNetworkSuite$

check-basic: .footloose-alpine.stamp
	MKE_PATH=$(realpath ../mke) go test -count=1 -v -timeout 5m github.com/Mirantis/mke/inttest/basic

check-hacontrolplane: .footloose-alpine.stamp
	MKE_PATH=$(realpath ../mke) go test -count=1 -v -timeout 3m github.com/Mirantis/mke/inttest/hacontrolplane

.PHONY: clean
clean:
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz .*.stamp


sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_linux_amd64.tar.gz

curl = curl -L --silent

bins = bin/sonobuoy

.PHONY: all
all: $(bins) id_ed25519_mke

bin:
	mkdir -p $@

bin/sonobuoy: | bin
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

check-network: bin/sonobuoy
	MKE_PATH=$(realpath ../mke) SONOBUOY_PATH=$(realpath bin/sonobuoy) \
		go test -count=1 -v -timeout 20m github.com/Mirantis/mke/inttest/sonobuoy/ -run ^TestNetworkSuite$

check-basic:
	MKE_PATH=$(realpath ../mke) go test -count=1 -v -timeout 20m github.com/Mirantis/mke/inttest/basic

check-etcd:
	MKE_PATH=$(realpath ../mke) go test -count=1 -v -timeout 20m github.com/Mirantis/mke/inttest/etcd

.PHONY: clean
clean:
	rm -rf bin sonobuoy/*_sonobuoy_*.tar.gz

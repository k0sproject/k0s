
RUNC_VERSION = 1.0.0-rc90
CONTAINERD_VERSION = 1.3.4
KUBLET_VERSION = 1.18.2

ARCH = amd64

all: build

bin/runc:
	mkdir -p $(dir $@)
	curl -L -o bin/runc https://github.com/opencontainers/runc/releases/download/v$(RUNC_VERSION)/runc.$(ARCH)

bin/containerd:
	mkdir -p $(dir $@)
	curl -L https://github.com/containerd/containerd/releases/download/v$(CONTAINERD_VERSION)/containerd-$(CONTAINERD_VERSION).linux-$(ARCH).tar.gz | tar zxvf -
	rm bin/containerd-stress
	rm bin/ctr

bin/kubelet:
	mkdir -p $(dir $@)
	curl -L -o bin/kubelet https://storage.googleapis.com/kubernetes-release/release/v$(KUBLET_VERSION)/bin/linux/$(ARCH)/kubelet

pkg/assets/zz_generated_bindata.go: bin/kubelet bin/containerd bin/runc
	go-bindata -o pkg/assets/zz_generated_bindata.go -pkg assets bin/

build: pkg/assets/zz_generated_bindata.go
	go build -o mke main.go

clean:
	rm -f pkg/assets/zz_generated_bindata.go mke
	rm -rf bin/

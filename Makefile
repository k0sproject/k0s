
RUNC_VERSION = 1.0.0-rc90
CONTAINERD_VERSION = 1.3.6
KUBE_VERSION = 1.18.5
KINE_VERSION = 0.4.0
ETCD_VERSION = 3.4.10

GO_SRCS := $(shell find -name '*.go')
TMPDIR ?= .tmp

ARCH = amd64

.PHONY: all
all: build

bin/runc:
	mkdir -p $(dir $@)
	curl --silent -L -o bin/runc https://github.com/opencontainers/runc/releases/download/v$(RUNC_VERSION)/runc.$(ARCH)

bin/containerd:
	mkdir -p $(dir $@)
	curl --silent -L https://github.com/containerd/containerd/releases/download/v$(CONTAINERD_VERSION)/containerd-$(CONTAINERD_VERSION)-linux-$(ARCH).tar.gz \
		| tar zxv bin/containerd bin/containerd-shim bin/containerd-shim-runc-v1 bin/containerd-shim-runc-v2

bin/kubelet:
	mkdir -p $(dir $@)
	curl --silent -L -o bin/kubelet https://storage.googleapis.com/kubernetes-release/release/v$(KUBE_VERSION)/bin/linux/$(ARCH)/kubelet

bin/kine:
	if ! [ -d $(TMPDIR)/kine ]; then \
		mkdir -p $(TMPDIR) \
			&& cd $(TMPDIR) \
			&& git clone -b v$(KINE_VERSION) --depth=1 https://github.com/rancher/kine.git; \
	fi
	cd $(TMPDIR)/kine && go build -o $(PWD)/$@

bin/etcd:
	mkdir -p $(dir $@)
	curl --silent -L https://github.com/etcd-io/etcd/releases/download/v$(ETCD_VERSION)/etcd-v$(ETCD_VERSION)-linux-$(ARCH).tar.gz \
		| tar -C bin/ -zxv --strip-components=1 etcd-v$(ETCD_VERSION)-linux-$(ARCH)/etcd

# k8s control plane components
# TODO We might be better of by getting hyperkube bin, but that doesn't seem to be downloadable from the release package URL :(
bin/kube-apiserver:
	mkdir -p $(dir $@)
	curl --silent -L -o bin/kube-apiserver https://storage.googleapis.com/kubernetes-release/release/v$(KUBE_VERSION)/bin/linux/$(ARCH)/kube-apiserver


bin/kube-scheduler:
	mkdir -p $(dir $@)
	curl --silent -L -o bin/kube-scheduler https://storage.googleapis.com/kubernetes-release/release/v$(KUBE_VERSION)/bin/linux/$(ARCH)/kube-scheduler

bin/kube-controller-manager:
	mkdir -p $(dir $@)
	curl --silent -L -o bin/kube-controller-manager https://storage.googleapis.com/kubernetes-release/release/v$(KUBE_VERSION)/bin/linux/$(ARCH)/kube-controller-manager


pkg/assets/zz_generated_bindata.go: bin/kube-scheduler bin/kube-apiserver bin/kube-controller-manager bin/kubelet bin/containerd bin/runc bin/kine bin/etcd
	go-bindata -o pkg/assets/zz_generated_bindata.go -pkg assets bin/

mke: pkg/assets/zz_generated_bindata.go $(GO_SRCS)
	go build -o mke main.go

.PHONY: build
build: mke

.PHONY: clean
clean:
	rm -f pkg/assets/zz_generated_bindata.go mke
	rm -rf bin/ $(TMPDIR)/kine
	rmdir $(TMPDIR) 2>/dev/null || true

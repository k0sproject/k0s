
runc_version = 1.0.0-rc92
containerd_version = 1.4.0
kubernetes_version = 1.19.3
kine_version = 0.4.1
etcd_version = 3.4.13
konnectivity_version = 0.0.12

bindir = staging/linux/bin
bins = runc kubelet containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2 kube-apiserver kube-scheduler kube-controller-manager etcd kine konnectivity-server

buildmode = docker

.PHONY: all
all: $(addprefix $(bindir)/, $(bins))

.PHONY: clean
clean:
	for i in .container.*; do \
		if [ -f $$i ]; then \
			docker rm $$(cat $$i); rm $$i; \
		fi; \
	done
	for i in .docker-image.*; do \
		if [ -f $$i ]; then \
			docker rmi mkebuild$$(basename $$i .stamp) ; rm $$i;\
		fi; \
	done
	rm -rf staging

$(bindir):
	mkdir -p $@

ifeq ($(buildmode),docker)

dockerfiles := $(wildcard */Dockerfile)
images := $(patsubst %/Dockerfile,%,$(dockerfiles))

containers: $(images:%=.container.%)
docker-images: $(images:%=.docker-image.%.stamp)

$(bindir)/runc: .container.runc
$(bindir)/containerd $(bindir)/containerd-shim $(bindir)/containerd-shim-runc-v1 $(bindir)/containerd-shim-runc-v2: .container.containerd
$(bindir)/etcd: .container.etcd
$(bindir)/kine: .container.kine
$(bindir)/konnectivity-server: .container.konnectivity
$(bindir)/kubelet $(bindir)/kube-apiserver $(bindir)/kube-scheduler $(bindir)/kube-controller-manager: .container.kubernetes

$(addprefix $(bindir)/, $(bins)): | $(bindir)
	docker export $$(cat $<) | tar -C $(dir $(bindir)) -xv bin/$(notdir $@) && touch $@

.container.%: .docker-image.%.stamp
	docker create mkebuild$(basename $<) > $@.tmp
	mv $@.tmp $@

.docker-image.%.stamp: %/Dockerfile
	docker build -t mkebuild$(basename $@) --build-arg VERSION=$($(patsubst %/Dockerfile,%,$<)_version) -f $< .
	touch $@

else
# fetch official binaires
#
curl = curl --silent -L

runc_url = https://github.com/opencontainers/runc/releases/download/v$(runc_version)/runc.$(arch)
kubelet_url = https://storage.googleapis.com/kubernetes-release/release/v$(kubernetes_version)/bin/linux/$(arch)/kubelet
kube-apiserver_url = https://storage.googleapis.com/kubernetes-release/release/v$(kubernetes_version)/bin/linux/$(arch)/kube-apiserver
kube-scheduler_url = https://storage.googleapis.com/kubernetes-release/release/v$(kubernetes_version)/bin/linux/$(arch)/kube-scheduler
kube-controller-manager_url = https://storage.googleapis.com/kubernetes-release/release/v$(kubernetes_version)/bin/linux/$(arch)/kube-controller-manager

containerd_url = https://github.com/containerd/containerd/releases/download/v$(containerd_version)/containerd-$(containerd_version)-linux-$(arch).tar.gz
etcd_url = https://github.com/etcd-io/etcd/releases/download/v$(etcd_version)/etcd-v$(etcd_version)-linux-$(arch).tar.gz

containerd_extract = bin/containerd bin/containerd-shim bin/containerd-shim-runc-v1 bin/containerd-shim-runc-v2
etcd_extract = etcd-v$(etcd_version)-linux-$(arch)/etcd

tmpdir ?= .tmp
arch = amd64


$(addprefix $(bindir)/, runc kubelet kube-apiserver kube-scheduler kube-controller-manager): | $(bindir)
	$(curl) -o $@ $($(notdir $@)_url)

$(addprefix $(bindir)/, containerd etcd): | $(bindir)
	$(curl) $($(notdir $@)_url) | tar -C $(bindir)/ -zxv --strip-components=1 $($(notdir $@)_extract)

# kine does not ship precompiled binaries so lets build it from source
$(bindir)/kine: | $(bindir)
	if ! [ -d $(tmpdir)/kine ]; then \
		mkdir -p $(tmpdir) \
			&& cd $(tmpdir) \
			&& git clone -b v$(kine_version) --depth=1 https://github.com/rancher/kine.git; \
	fi
	cd $(tmpdir)/kine && go build -o $(CURDIR)/$@

# konnectivity does not ship precompiled binaries so lets build it from source
$(bindir)/konnectivity-server: | $(bindir)
	if ! [ -d $(tmpdir)/apiserver-network-proxy ]; then \
		mkdir -p $(tmpdir) \
			&& cd $(tmpdir) \
			&& git clone -b v$(konnectivity_version) --depth=1 https://github.com/kubernetes-sigs/apiserver-network-proxy.git; \
	fi
	cd $(tmpdir)/apiserver-network-proxy && make bin/proxy-server && cp bin/proxy-server $(CURDIR)/$@

endif

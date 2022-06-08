include Makefile.variables

GOOS ?= linux
export GOOS

TARGET_OS ?= linux
export TARGET_OS

bindir = staging/${TARGET_OS}/bin
posix_bins = runc kubelet containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2 kube-apiserver kube-scheduler kube-controller-manager etcd kine konnectivity-server xtables-legacy-multi
buildmode = docker

bins = ${posix_bins}

.PHONY: all
all: $(addprefix $(bindir)/, $(bins))

.PHONY: clean
clean:
	for i in .container.*; do \
		if [ -f $$i ]; then \
			docker rm $$(cat $$i) && rm $$i; \
		fi; \
	done
	for i in .docker-image.*; do \
		if [ -f $$i ]; then \
			docker rmi -f k0sbuild$$(basename $$i .stamp) && rm $$i;\
		fi; \
	done
	rm -rf staging .tmp/* *.tmp

$(bindir):
	mkdir -p $@

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
$(bindir)/xtables-legacy-multi: .container.iptables

$(addprefix $(bindir)/, $(bins)): | $(bindir)
	docker export $$(cat $<) | tar -C $(dir $(bindir)) -xv bin/$(notdir $@) && touch $@

.container.%: .docker-image.%.stamp
	docker create k0sbuild$(basename $<) > $@.tmp
	mv $@.tmp $@

.docker-image.%.stamp: %/Dockerfile Makefile.variables
	docker build -t k0sbuild$(basename $@) \
		--build-arg VERSION=$($(patsubst %/Dockerfile,%,$<)_version) \
		--build-arg BUILDIMAGE=$($(patsubst %/Dockerfile,%,$<)_buildimage) \
		--build-arg BUILD_GO_TAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_tags) \
		--build-arg BUILD_GO_CGO_ENABLED=$($(patsubst %/Dockerfile,%,$<)_build_go_cgo_enabled) \
		--build-arg BUILD_SHIM_GO_CGO_ENABLED=$($(patsubst %/Dockerfile,%,$<)_build_shim_go_cgo_enabled) \
		--build-arg BUILD_GO_FLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_flags) \
		--build-arg BUILD_GO_LDFLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_ldflags) \
		--build-arg BUILD_GO_LDFLAGS_EXTRA=$($(patsubst %/Dockerfile,%,$<)_build_go_ldflags_extra) \
		-f $< .
	touch $@
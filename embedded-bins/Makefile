include Makefile.variables

GOOS ?= linux
export GOOS

TARGET_OS ?= linux
export TARGET_OS

bindir = staging/${TARGET_OS}/bin
posix_bins = runc kubelet containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2 kube-apiserver kube-scheduler kube-controller-manager etcd kine konnectivity-server xtables-legacy-multi xtables-nft-multi
windows_bins = kubelet.exe kube-proxy.exe
buildmode = docker

ifeq ($(TARGET_OS),windows)
bins = ${windows_bins}
else
bins = ${posix_bins}
endif

.PHONY: all
all: $(addprefix $(bindir)/, $(bins))

.PHONY: clean
clean:
	for i in .container.*; do \
	  [ -f "$$i" ] || continue; \
	  docker rm -- $$(cat -- "$$i") && rm -- "$$i"; \
	done
	for i in .docker-image.*.stamp; do \
	  [ -f "$$i" ] || continue; \
	  tags=$$(docker inspect --format='{{range $$tag := .RepoTags}}{{$$tag}} {{end}}' -- $$(cat -- "$$i")) && \
	  docker rmi -f -- $$tags && \
	  rm -f -- "$$i"; \
	done
	rm -rf staging
	@echo 'cleaned up; you may want to run `docker system prune` in order to free dangling resources ...'

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
$(bindir)/xtables-nft-multi: .container.iptables

$(bindir)/kubelet.exe $(bindir)/kube-proxy.exe: .container.kubernetes.windows

$(addprefix $(bindir)/, $(bins)): | $(bindir)
	docker export $$(cat $<) | tar -C $(dir $(bindir)) -xv bin/$(notdir $@) && touch $@

build_docker_container = \
	docker create --entrypoint=/dev/null $(shell cat -- $<) > $@.tmp \
	&& mv $@.tmp $@

.container.%: .docker-image.%.stamp
	$(build_docker_container)

.container.%.windows: .docker-image.%.windows.stamp
	$(build_docker_container)

build_docker_image = \
	docker build -t k0sbuild$(basename $@):latest \
	  --build-arg TARGET_OS=$(if $(findstring .windows.stamp,$@),windows,linux) \
	  --build-arg VERSION=$($(patsubst %/Dockerfile,%,$<)_version) \
	  --build-arg BUILDIMAGE=$($(patsubst %/Dockerfile,%,$<)_buildimage) \
	  --build-arg BUILD_GO_TAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_tags) \
	  --build-arg BUILD_GO_CGO_ENABLED=$($(patsubst %/Dockerfile,%,$<)_build_go_cgo_enabled) \
	  --build-arg BUILD_GO_CGO_CFLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_cgo_cflags) \
	  --build-arg BUILD_SHIM_GO_CGO_ENABLED=$($(patsubst %/Dockerfile,%,$<)_build_shim_go_cgo_enabled) \
	  --build-arg BUILD_GO_FLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_flags) \
	  --build-arg BUILD_GO_LDFLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_ldflags) \
	  --build-arg BUILD_GO_LDFLAGS_EXTRA=$($(patsubst %/Dockerfile,%,$<)_build_go_ldflags_extra) \
	  -- $(dir $<) \
	&& docker images -q k0sbuild$(basename $@):latest > $@.tmp \
	&& mv $@.tmp $@

.docker-image.%.stamp: %/Dockerfile Makefile.variables
	$(build_docker_image)

.docker-image.%.windows.stamp: %/Dockerfile Makefile.variables
	$(build_docker_image)

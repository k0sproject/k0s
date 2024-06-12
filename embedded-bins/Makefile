include Makefile.variables

GOOS ?= linux
export GOOS

TARGET_OS ?= linux
export TARGET_OS

# https://reproducible-builds.org/docs/source-date-epoch/#makefile
# https://reproducible-builds.org/docs/source-date-epoch/#git
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct || date -u +%s)

bindir = staging/${TARGET_OS}/bin
posix_bins = runc kubelet containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2 kube-apiserver kube-scheduler kube-controller-manager etcd kine konnectivity-server xtables-legacy-multi xtables-nft-multi keepalived
windows_bins = kubelet.exe kube-proxy.exe containerd.exe containerd-shim-runhcs-v1.exe

ifeq ($(TARGET_OS),windows)
bins = ${windows_bins}
# suffix .exe is added to the binaries in the original build pipelines
containerd_bins = containerd containerd-shim-runhcs-v1
kubernetes_bins = kubelet kube-proxy
else
containerd_bins = containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2
kubernetes_bins = kubelet kube-apiserver kube-scheduler kube-controller-manager
bins = ${posix_bins}
endif

.DELETE_ON_ERROR:

.PHONY: all
all: $(addprefix $(bindir)/, $(bins))

.PHONY: clean
clean: IID_FILES = .docker-image.*.stamp
clean:
	for i in .container.*; do \
	  [ -f "$$i" ] || continue; \
	  docker rm -- "$$(cat -- "$$i")"; \
	  rm -- "$$i"; \
	done
	$(clean-iid-files)
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
$(bindir)/keepalived: .container.keepalived

$(bindir)/kubelet.exe $(bindir)/kube-proxy.exe: .container.kubernetes.windows 
$(bindir)/containerd.exe $(bindir)/containerd-shim-runhcs-v1.exe: .container.containerd.windows

$(addprefix $(bindir)/, $(bins)): | $(bindir)
	tardir=$$(mktemp -d -- '.$(notdir $@).XXXXXX.tmp') \
	  && trap "rm -rf -- $$tardir" INT EXIT \
	  && docker export "$$(cat $<)" \
	  | tar -C "$$tardir" -x bin/$(notdir $@) \
	  && touch -c -- "$$tardir"/bin/$(notdir $@) \
	  && mv "$$tardir"/bin/$(notdir $@) $@

build_docker_container = \
	rm -f -- '$@'; \
	docker create --cidfile='$@' --entrypoint=/dev/null '$(shell cat -- $<)'

.container.%: .docker-image.%.stamp
	$(build_docker_container)

.container.%.windows: .docker-image.%.windows.stamp
	$(build_docker_container)

build_docker_image = \
	docker build --progress=plain --iidfile '$@' -t k0sbuild$(basename $@):latest \
	  --build-arg TARGET_OS=$(if $(findstring .windows.stamp,$@),windows,linux) \
	  --build-arg CONTAINERD_BINS="$(containerd_bins)" \
	  --build-arg KUBERNETES_BINS="$(kubernetes_bins)" \
	  --build-arg VERSION=$($(patsubst %/Dockerfile,%,$<)_version) \
	  --build-arg SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) \
	  --build-arg BUILDIMAGE=$($(patsubst %/Dockerfile,%,$<)_buildimage) \
	  --build-arg BUILD_GO_TAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_tags) \
	  --build-arg BUILD_GO_CGO_ENABLED=$($(patsubst %/Dockerfile,%,$<)_build_go_cgo_enabled) \
	  --build-arg BUILD_GO_CGO_CFLAGS='$($(patsubst %/Dockerfile,%,$<)_build_go_cgo_cflags)' \
	  --build-arg BUILD_SHIM_GO_CGO_ENABLED=$($(patsubst %/Dockerfile,%,$<)_build_shim_go_cgo_enabled) \
	  --build-arg BUILD_GO_FLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_flags) \
	  --build-arg BUILD_GO_LDFLAGS=$($(patsubst %/Dockerfile,%,$<)_build_go_ldflags) \
	  --build-arg BUILD_GO_LDFLAGS_EXTRA=$($(patsubst %/Dockerfile,%,$<)_build_go_ldflags_extra) \
	  -- $(dir $<)

.docker-image.%.stamp: %/Dockerfile Makefile.variables
	$(build_docker_image)

.docker-image.%.windows.stamp: %/Dockerfile Makefile.variables
	$(build_docker_image)

ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

ARG VERSION
ARG BUILD_GO_TAGS
ARG BUILD_GO_CGO_ENABLED
ARG BUILD_GO_FLAGS
ARG BUILD_GO_LDFLAGS
ARG BUILD_GO_LDFLAGS_EXTRA
ENV GOPATH=/go
ENV COMMANDS="kubelet kube-apiserver kube-scheduler kube-controller-manager"

RUN apk add build-base git go-bindata linux-headers rsync grep coreutils bash

RUN mkdir -p $GOPATH/src/github.com/kubernetes/kubernetes
RUN git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/kubernetes/kubernetes.git $GOPATH/src/github.com/kubernetes/kubernetes
WORKDIR /go/src/github.com/kubernetes/kubernetes

RUN go version
RUN \
	set -e; \
	# Explicitly set the Kubernetes version information. The auto-detection \
	# adds a plus to the minor git version when it detects an alpha/beta/rc \
	# release, because k0s adds its +k0s suffix to the version number. The \
	# plus would indicate that it's not the exact tagged version, but that the \
	# tree is between two tagged versions. This is not true for k0s-built \
	# kubernetes binaries. \
	# https://github.com/kubernetes/kubernetes/blob/v1.25.0-rc.0/hack/lib/version.sh#L98-L107 \
	# \
	{ \
		echo KUBE_GIT_COMMIT="'$(git rev-parse 'HEAD^{commit}')'"; \
		echo KUBE_GIT_TREE_STATE=clean; \
		echo KUBE_GIT_VERSION="'v$VERSION+k0s'"; \
		gitMajor=${VERSION%%.*}; \
		gitMinor=${VERSION#$gitMajor.}; \
		gitMinor=${gitMinor%%.*}; \
		echo KUBE_GIT_MAJOR="'$gitMajor'"; \
		echo KUBE_GIT_MINOR="'$gitMinor'"; \
	} > /kube_version; \
	# Ensure that all of the binaries are built with CGO \
	if [ ${BUILD_GO_CGO_ENABLED:-0} -eq 1 ]; then \
		export KUBE_CGO_OVERRIDES="${COMMANDS}"; \
	fi; \
	for cmd in $COMMANDS; do \
		export KUBE_GIT_VERSION_FILE=/kube_version; \
		make GOFLAGS="${BUILD_GO_FLAGS} -tags=${BUILD_GO_TAGS}" GOLDFLAGS="${BUILD_GO_LDFLAGS_EXTRA}" WHAT=cmd/$cmd; \
	done

FROM scratch
COPY --from=build \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kubelet \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kube-apiserver \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kube-scheduler \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kube-controller-manager \
	/bin/
CMD ["/bin/kubelet"]

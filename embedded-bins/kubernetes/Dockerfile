ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

ARG VERSION
RUN apk add build-base git linux-headers rsync grep coreutils bash

RUN mkdir -p $GOPATH/src/github.com/kubernetes/kubernetes
RUN git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/kubernetes/kubernetes.git $GOPATH/src/github.com/kubernetes/kubernetes
WORKDIR /go/src/github.com/kubernetes/kubernetes

ARG TARGET_OS \
  SOURCE_DATE_EPOCH \
  BUILD_GO_TAGS \
  BUILD_GO_CGO_ENABLED \
  BUILD_GO_FLAGS \
  BUILD_GO_LDFLAGS \
  BUILD_GO_LDFLAGS_EXTRA \
  KUBERNETES_BINS

RUN \
  set -ex; \
  export GOPATH=/go; \
  if [ "${TARGET_OS}" = windows ]; then \
    commands="${KUBERNETES_BINS}"; \
    binarySuffix=.exe; \
    export KUBE_BUILD_PLATFORMS=windows/amd64; \
  else \
    commands="${KUBERNETES_BINS}"; \
    binarySuffix=''; \
  fi; \
  # Ensure that all of the binaries are built with CGO \
  if [ ${BUILD_GO_CGO_ENABLED:-0} -eq 1 ]; then \
    export KUBE_CGO_OVERRIDES=$commands; \
  else \
    export KUBE_STATIC_OVERRIDES=$commands; \
  fi; \
  mkdir /out; \
  export SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH; \
  export FORCE_HOST_GO=y; \
  export KUBE_VERBOSE=9; \
  export KUBE_GIT_VERSION="v$VERSION+k0s"; \
  for cmd in $commands; do \
    make GOFLAGS="${BUILD_GO_FLAGS} -tags=${BUILD_GO_TAGS}" GOLDFLAGS="${BUILD_GO_LDFLAGS_EXTRA}" WHAT=cmd/$cmd; \
    mv /go/src/github.com/kubernetes/kubernetes/_output/local/bin/*/*/$cmd$binarySuffix /out/; \
  done

FROM scratch
COPY --from=build out/* /bin/

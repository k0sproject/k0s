ARG BUILDIMAGE
FROM $BUILDIMAGE

ARG TARGETARCH
RUN set -ex; \
# Need to use the gold linker on ARM, Go really wants to have it.
# https://github.com/golang/go/blob/go1.23.2/src/cmd/link/internal/ld/lib.go##L1661-L1680
  case "$TARGETARCH" in \
  arm*) binutils=binutils-gold ;; \
    *)    binutils=binutils ;; \
  esac; \
  apk add --no-cache make gcc musl-dev "$binutils"

ENV \
  HOME="/run/k0s-build" \
  PATH="/run/k0s-build/go/bin:$PATH" \
  GOBIN="/run/k0s-build/go/bin" \
  GOCACHE="/run/k0s-build/go/build" \
  GOMODCACHE="/run/k0s-build/go/mod"

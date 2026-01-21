ARG BUILDIMAGE
FROM $BUILDIMAGE

ARG TARGETARCH
RUN set -ex; \
# Need to use the gold linker on ARM, Go really wants to have it.
# https://github.com/golang/go/blob/go1.24.12/src/cmd/link/internal/ld/lib.go#L1674-L1693
  case "$TARGETARCH" in \
  arm*) binutils=binutils-gold ;; \
    *)    binutils=binutils ;; \
  esac; \
  if [ ! -z "$(which apt)" ]; then \
     apt update && apt install -y make gcc; \
  elif [ ! -z "$(which apk)" ]; then \
     apk add --no-cache make gcc musl-dev "$binutils"; \
  else \
     echo "unsupported package manager"; \
     exit 1; \
  fi


ENV \
  HOME="/run/k0s-build" \
  PATH="/run/k0s-build/go/bin:$PATH" \
  GOBIN="/run/k0s-build/go/bin" \
  GOCACHE="/run/k0s-build/go/build" \
  GOMODCACHE="/run/k0s-build/go/mod"

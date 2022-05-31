ARG BUILDIMAGE
FROM $BUILDIMAGE
RUN apk add --no-cache make gcc musl-dev binutils-gold

ENV \
  HOME="/run/k0s-build" \
  PATH="/run/k0s-build/go/bin:$PATH" \
  GOBIN="/run/k0s-build/go/bin" \
  GOCACHE="/run/k0s-build/go/build" \
  GOMODCACHE="/run/k0s-build/go/mod"

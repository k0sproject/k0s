ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN if [ ! -z "$(which apt)" ]; then \
       apt update && apt install -y git; \
    elif [ ! -z "$(which apk)" ]; then \
       apk add git; \
    else \
       echo "unsupported package manager"; \
       exit 1; \
    fi

ARG VERSION
WORKDIR /apiserver-network-proxy
RUN git -c advice.detachedHead=false clone --revision refs/tags/v$VERSION --depth=1 https://github.com/kubernetes-sigs/apiserver-network-proxy.git .
ARG REVISION
RUN set -eu \
  && if [ -n "$REVISION" ]; then \
    actual=$(git rev-parse HEAD); \
    [ "$actual" = "$REVISION" ] || { \
      printf 'error: tag v%s resolves to %s, expected %s\n' "$VERSION" "$actual" "$REVISION" >&2; \
      exit 1; \
    }; \
  fi

ARG TARGET_OS \
  BUILD_GO_TAGS \
  BUILD_GO_CGO_ENABLED \
  BUILD_GO_FLAGS \
  BUILD_GO_LDFLAGS

RUN CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    GOOS=linux \
    go build \
        ${BUILD_GO_FLAGS} \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS}" \
        -o bin/proxy-server cmd/server/main.go

FROM scratch
COPY --from=build /apiserver-network-proxy/bin/proxy-server /bin/konnectivity-server

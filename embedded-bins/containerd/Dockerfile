ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

ENV GOPATH=/go

RUN if [ ! -z "$(which apt)" ]; then \
       apt update && apt install -y \
        build-essential git \
        btrfs-progs protobuf-compiler \
        bash; \
    elif [ ! -z "$(which apk)" ]; then \
       apk upgrade -U -a && apk add \
        build-base git \
        btrfs-progs-dev btrfs-progs-static \
        protoc bash; \
    else \
       echo "unsupported package manager"; \
       exit 1; \
    fi

ARG VERSION
WORKDIR /go/src/github.com/containerd/containerd
RUN git -c advice.detachedHead=false clone --revision refs/tags/v$VERSION --depth=1 https://github.com/containerd/containerd.git .
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
  BUILD_SHIM_GO_CGO_ENABLED \
  BUILD_GO_FLAGS \
  BUILD_GO_LDFLAGS \
  CONTAINERD_BINS

RUN make \
	CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
	SHIM_CGO_ENABLED=${BUILD_SHIM_GO_CGO_ENABLED} \
	GO_BUILDTAGS="${BUILD_GO_TAGS}" \
	COMMANDS="${CONTAINERD_BINS}" \
	GO_BUILD_FLAGS="${BUILD_GO_FLAGS}" \
	EXTRA_LDFLAGS="${BUILD_GO_LDFLAGS}" \
	GOOS="${TARGET_OS}"

FROM scratch
COPY --from=build /go/src/github.com/containerd/containerd/bin/* /bin/

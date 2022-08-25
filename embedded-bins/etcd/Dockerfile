ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base git

ARG VERSION
RUN cd / && git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/etcd-io/etcd.git
WORKDIR /etcd/server
RUN go version

ARG TARGET_OS \
  BUILD_GO_TAGS \
  BUILD_GO_CGO_ENABLED \
  BUILD_GO_FLAGS \
  BUILD_GO_LDFLAGS \
  BUILD_GO_LDFLAGS_EXTRA

RUN CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    go build \
        ${BUILD_GO_FLAGS} \
	-installsuffix=cgo \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS} ${BUILD_GO_LDFLAGS_EXTRA} -X=go.etcd.io/etcd/api/v3/version.GitSHA=$(git rev-parse --short HEAD || echo "GitNotFound")" \
        -o /bin/etcd

FROM scratch
COPY --from=build /bin/etcd /bin/etcd

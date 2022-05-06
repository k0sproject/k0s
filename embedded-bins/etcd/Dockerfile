ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

ARG VERSION
ARG BUILD_GO_TAGS
ARG BUILD_GO_CGO_ENABLED
ARG BUILD_GO_FLAGS
ARG BUILD_GO_LDFLAGS
ARG BUILD_GO_LDFLAGS_EXTRA

RUN apk add build-base git

RUN cd / && git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/etcd-io/etcd.git
WORKDIR /etcd/server
RUN go version
RUN CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    go build \
        ${BUILD_GO_FLAGS} \
	-installsuffix=cgo \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS} ${BUILD_GO_LDFLAGS_EXTRA} -X=go.etcd.io/etcd/api/v3/version.GitSHA=$(git rev-parse --short HEAD || echo "GitNotFound")" \
        -o /bin/etcd

FROM scratch
COPY --from=build /bin/etcd /bin/etcd
CMD ["/bin/etcd"]

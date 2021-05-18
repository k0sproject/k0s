ARG BUILDIMAGE=golang:1.13-alpine
FROM $BUILDIMAGE AS build

ARG VERSION
ARG BUILD_GO_TAGS
ARG BUILD_GO_CGO_ENABLED
ARG BUILD_GO_FLAGS
ARG BUILD_GO_LDFLAGS
ARG BUILD_GO_LDFLAGS_EXTRA

RUN apk add build-base git

RUN cd / && git clone -b v$VERSION --depth=1 https://github.com/etcd-io/etcd.git
WORKDIR /etcd
RUN go version
RUN go mod vendor
RUN CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    go build \
        ${BUILD_GO_FLAGS} \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS} ${BUILD_GO_LDFLAGS_EXTRA}" \
        -o etcd

FROM scratch
COPY --from=build /etcd/etcd /bin/etcd
CMD ["/bin/etcd"]

ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base git make protoc

ARG VERSION
RUN git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/k0sproject/apiserver-network-proxy.git /apiserver-network-proxy
WORKDIR /apiserver-network-proxy

ARG TARGET_OS \
  BUILD_GO_TAGS \
  BUILD_GO_CGO_ENABLED \
  BUILD_GO_FLAGS \
  BUILD_GO_LDFLAGS \
  BUILD_GO_LDFLAGS_EXTRA

RUN go version
RUN go install github.com/golang/mock/mockgen@v1.4.4 && \
    go install github.com/golang/protobuf/protoc-gen-go@v1.4.3 && \
    make gen && \
    CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    GOOS=linux \
    go build \
        ${BUILD_GO_FLAGS} \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS} ${BUILD_GO_LDFLAGS_EXTRA}" \
        -o bin/proxy-server cmd/server/main.go

FROM scratch
COPY --from=build /apiserver-network-proxy/bin/proxy-server /bin/konnectivity-server

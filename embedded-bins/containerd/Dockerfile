ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

ARG VERSION
ARG BUILD_GO_TAGS
ARG BUILD_GO_CGO_ENABLED
ARG BUILD_SHIM_GO_CGO_ENABLED
ARG BUILD_GO_FLAGS
ARG BUILD_GO_LDFLAGS
ARG BUILD_GO_LDFLAGS_EXTRA
ENV GOPATH=/go

RUN apk upgrade -U -a && apk add \
	build-base git \
	btrfs-progs-dev btrfs-progs-static \
	protoc

RUN mkdir -p $GOPATH/src/github.com/containerd/containerd
RUN git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/containerd/containerd.git $GOPATH/src/github.com/containerd/containerd
WORKDIR /go/src/github.com/containerd/containerd
RUN go version
RUN make \
	CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
	SHIM_CGO_ENABLED=${BUILD_SHIM_GO_CGO_ENABLED} \
	GO_TAGS="-tags=${BUILD_GO_TAGS}" \
	COMMANDS="containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2" \
	GO_BUILD_FLAGS="${BUILD_GO_FLAGS}" \
	EXTRA_LDFLAGS="${BUILD_GO_LDFLAGS_EXTRA}"

FROM scratch
COPY --from=build /go/src/github.com/containerd/containerd/bin/* /bin/
CMD ["/bin/containerd"]

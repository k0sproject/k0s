ARG BUILDIMAGE=golang:1.15-alpine
FROM $BUILDIMAGE AS build

ARG VERSION
ENV GOPATH=/go

RUN apk upgrade -U -a && apk add \
	build-base git \
	btrfs-progs-dev btrfs-progs-static \
	protoc

RUN mkdir -p $GOPATH/src/github.com/containerd/containerd
RUN git clone -b v$VERSION --depth=1 https://github.com/containerd/containerd.git $GOPATH/src/github.com/containerd/containerd
WORKDIR /go/src/github.com/containerd/containerd
RUN make COMMANDS="containerd containerd-shim containerd-shim-runc-v1 containerd-shim-runc-v2" \
	EXTRA_LDFLAGS="-extldflags=-static -s -w"

FROM scratch
COPY --from=build /go/src/github.com/containerd/containerd/bin/* /bin/
CMD ["/bin/containerd"]

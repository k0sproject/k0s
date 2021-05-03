ARG BUILDIMAGE=golang:1.15-alpine
FROM $BUILDIMAGE AS build

ARG VERSION

RUN apk add build-base git

RUN cd / && git clone -b v$VERSION --depth=1 https://github.com/kubernetes-sigs/cri-tools.git
WORKDIR /cri-tools
RUN CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -o bin/crictl -ldflags "'-extldflags=-static -w -s'" github.com/kubernetes-sigs/cri-tools/cmd/crictl
FROM scratch
COPY --from=build /cri-tools/bin/crictl /bin/crictl
CMD ["/bin/crictl"]

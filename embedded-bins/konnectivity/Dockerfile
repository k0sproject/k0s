ARG BUILDIMAGE=golang:1.15-alpine
FROM $BUILDIMAGE AS build

ARG VERSION

RUN apk add build-base git make

RUN git clone -b v$VERSION --depth=1 https://github.com/kubernetes-sigs/apiserver-network-proxy.git /apiserver-network-proxy
WORKDIR /apiserver-network-proxy
RUN GO111MODULE=on go get github.com/golang/mock/mockgen@v1.4.4 && \
    make gen && \
    CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static -w -s"' -o bin/proxy-server cmd/server/main.go

FROM scratch
COPY --from=build /apiserver-network-proxy/bin/proxy-server /bin/konnectivity-server
CMD ["/bin/konnectivity-server"]

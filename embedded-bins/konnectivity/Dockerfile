FROM golang:1.15-alpine AS build

ARG VERSION

RUN apk add build-base git go make

RUN git clone -b v$VERSION --depth=1 https://github.com/kubernetes-sigs/apiserver-network-proxy.git /apiserver-network-proxy
WORKDIR /apiserver-network-proxy
RUN make GOFLAGS="-v" GOLDFLAGS="-extldflags='-static -w -s'" bin/proxy-server

FROM scratch
COPY --from=build /apiserver-network-proxy/bin/proxy-server /bin/konnectivity-server
CMD ["/bin/konnectivity-server"]

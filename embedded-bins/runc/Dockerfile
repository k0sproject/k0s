FROM alpine:edge AS build

ENV VERSION=1.0.0-rc92

ENV GOPATH=/go

RUN apk upgrade -U -a && apk add build-base git go \
	libseccomp-dev libseccomp-static

RUN mkdir -p $GOPATH/src/github.com/opencontainers/runc
RUN git clone -b v$VERSION --depth=1 https://github.com/opencontainers/runc.git $GOPATH/src/github.com/opencontainers/runc
WORKDIR /go/src/github.com/opencontainers/runc
RUN make EXTRA_LDFLAGS="-extldflags=-static"

FROM scratch
COPY --from=build /go/src/github.com/opencontainers/runc/runc /bin/runc
CMD ["/bin/runc"]

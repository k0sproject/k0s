FROM golang:1.15-alpine AS build

ARG VERSION
ARG LIBSECCOMP_VERSION=2.5.0

ENV GOPATH=/go

RUN apk add build-base git \
	curl linux-headers gperf bash pkgconf

RUN curl -L https://github.com/seccomp/libseccomp/releases/download/v$LIBSECCOMP_VERSION/libseccomp-$LIBSECCOMP_VERSION.tar.gz \
	| tar -C / -zx

RUN cd /libseccomp-$LIBSECCOMP_VERSION && ./configure --sysconfdir=/etc --enable-static

RUN make -j$(nproc) -C /libseccomp-$LIBSECCOMP_VERSION
RUN make -j$(nproc) -C /libseccomp-$LIBSECCOMP_VERSION check
RUN make -C /libseccomp-$LIBSECCOMP_VERSION install

RUN mkdir -p $GOPATH/src/github.com/opencontainers/runc
RUN git clone -b v$VERSION --depth=1 https://github.com/opencontainers/runc.git $GOPATH/src/github.com/opencontainers/runc
WORKDIR /go/src/github.com/opencontainers/runc
RUN make EXTRA_LDFLAGS="-extldflags=-static -w -s"

FROM scratch
COPY --from=build /go/src/github.com/opencontainers/runc/runc /bin/runc
CMD ["/bin/runc"]

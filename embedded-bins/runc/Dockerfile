ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base git \
	curl linux-headers gperf bash pkgconf

ENV GOPATH=/go

ARG LIBSECCOMP_VERSION=2.5.4
RUN curl -L https://github.com/seccomp/libseccomp/releases/download/v$LIBSECCOMP_VERSION/libseccomp-$LIBSECCOMP_VERSION.tar.gz \
	| tar -C / -zx

RUN cd /libseccomp-$LIBSECCOMP_VERSION && ./configure --sysconfdir=/etc --enable-static

RUN make -j$(nproc) -C /libseccomp-$LIBSECCOMP_VERSION
RUN make -j$(nproc) -C /libseccomp-$LIBSECCOMP_VERSION check
RUN make -C /libseccomp-$LIBSECCOMP_VERSION install

ARG VERSION
RUN mkdir -p $GOPATH/src/github.com/opencontainers/runc
RUN git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/opencontainers/runc.git $GOPATH/src/github.com/opencontainers/runc
WORKDIR /go/src/github.com/opencontainers/runc

ARG TARGET_OS \
  BUILD_GO_TAGS \
  BUILD_GO_CGO_ENABLED \
  BUILD_GO_FLAGS \
  BUILD_GO_LDFLAGS \
  BUILD_GO_LDFLAGS_EXTRA

RUN go version
RUN make \
	CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
	BUILDTAGS="${BUILD_GO_TAGS}" \
	EXTRA_FLAGS="${BUILD_GO_FLAGS}" \
	EXTRA_LDFLAGS="${BUILD_GO_LDFLAGS_EXTRA}"

FROM scratch
COPY --from=build /go/src/github.com/opencontainers/runc/runc /bin/runc

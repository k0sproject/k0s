ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base git \
	curl linux-headers gperf bash pkgconf cmd:tail

ENV GOPATH=/go

ARG LIBSECCOMP_VERSION=2.5.5
RUN curl --proto '=https' --tlsv1.2 -L https://github.com/seccomp/libseccomp/releases/download/v$LIBSECCOMP_VERSION/libseccomp-$LIBSECCOMP_VERSION.tar.gz \
	| tar -C / -zx

WORKDIR /libseccomp-$LIBSECCOMP_VERSION
# tests: add basic support for running tests in parallel
RUN curl -L https://github.com/seccomp/libseccomp/commit/2380f5788c692796f75e464c61aa877e5c4eb882.patch \
	| git apply
# tests: limit the number of bpf-sim-fuzz test iterations
# https://github.com/seccomp/libseccomp/commit/5878cf2383ccedca3536f47155b13145809ae08e
# the patch didnt apply due to commit touches .travis/
# Use sed instead
RUN sed -i -e 's/50$/5/' tests/*-sim-*.tests

# tests: add support for the LIBSECCOMP_TSTCFG_JOBS env variable
RUN curl -L https://github.com/seccomp/libseccomp/commit/4c19425fa69cfb4f7de5225d676a26ef0b442e28.patch \
	| git apply
RUN ./configure --sysconfdir=/etc --enable-static

RUN make -j$(nproc)
RUN make -j$(nproc) check-build
RUN make -C tests check LIBSECCOMP_TSTCFG_JOBS=$(nproc)
RUN make install

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

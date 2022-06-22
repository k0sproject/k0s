ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

ARG VERSION
ARG BUILD_GO_TAGS
ARG BUILD_GO_CGO_ENABLED
ARG BUILD_GO_CGO_CFLAGS
ARG BUILD_GO_FLAGS
ARG BUILD_GO_LDFLAGS
ARG BUILD_GO_LDFLAGS_EXTRA

RUN apk add build-base git


RUN cd / && git -c advice.detachedHead=false clone -b v$VERSION --depth=1 https://github.com/rancher/kine.git
WORKDIR /kine
RUN go version
RUN CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    CGO_CFLAGS=${BUILD_GO_CGO_CFLAGS} go build \
        ${BUILD_GO_FLAGS} \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS} ${BUILD_GO_LDFLAGS_EXTRA}" \
        -o kine

FROM scratch
COPY --from=build /kine/kine /bin/kine
CMD ["/bin/kine"]

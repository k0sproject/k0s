ARG BUILDIMAGE=golang:1.18.1-alpine
FROM ${BUILDIMAGE} as build

ARG VERSION
ARG BUILD_GO_TAGS="osusergo,netgo"
ARG BUILD_GO_CGO_ENABLED=1
ARG BUILD_GO_FLAGS="-buildmode=pie"
ARG BUILD_GO_LDFLAGS="-w -extldflags=-static"
ARG BUILD_GO_LDFLAGS_EXTRA

RUN apk add binutils-gold build-base git

WORKDIR /go/src/github.com/k0sproject/autopilot
ADD go.mod go.sum ./

RUN go mod download

COPY cmd ./cmd/
COPY embedded ./embedded/
COPY internal ./internal/
COPY pkg ./pkg/

RUN CGO_ENABLED=${BUILD_GO_CGO_ENABLED} \
    go build \
        ${BUILD_GO_FLAGS} \
        -tags="${BUILD_GO_TAGS}" \
        -ldflags="${BUILD_GO_LDFLAGS} ${BUILD_GO_LDFLAGS_EXTRA}" \
        -o autopilot \
        ./cmd/autopilot


FROM scratch
COPY --from=build /go/src/github.com/k0sproject/autopilot/autopilot /autopilot/bin/autopilot
ENTRYPOINT ["/autopilot/bin/autopilot"]

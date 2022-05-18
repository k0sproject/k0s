ARG VERSION_CONTROLLERGEN=v0.8.0
ARG VERSION_CLIENTGEN=v0.24.0
ARG VERSION_GOBINDATA=latest
ARG VERSION_GOLINT=v1.46.2

FROM golang:1.18-alpine AS build-base
RUN apk add build-base git

FROM build-base AS build-clientgen
ARG VERSION_CLIENTGEN
RUN go install k8s.io/code-generator/cmd/client-gen@${VERSION_CLIENTGEN}

FROM build-base AS build-controllergen
ARG VERSION_CONTROLLERGEN
RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@${VERSION_CONTROLLERGEN}

FROM build-base AS build-gobindata
ARG VERSION_GOBINDATA
RUN go install github.com/kevinburke/go-bindata/go-bindata@${VERSION_GOBINDATA}

FROM build-base AS build-golint
ARG VERSION_GOLINT
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@${VERSION_GOLINT}

FROM build-base AS final
COPY --from=build-clientgen /go/bin/client-gen /go/bin
COPY --from=build-controllergen /go/bin/controller-gen /go/bin
COPY --from=build-gobindata /go/bin/go-bindata /go/bin
COPY --from=build-golint /go/bin/golangci-lint /go/bin
RUN mkdir -p /.cache/go-build
RUN chmod -R 777 /go /.cache
WORKDIR /go/src/k0s

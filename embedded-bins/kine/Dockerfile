FROM alpine:3.12 AS build

ARG VERSION

RUN apk add build-base git go

RUN git clone -b v$VERSION --depth=1 https://github.com/rancher/kine.git
WORKDIR /kine
RUN go build -ldflags="-extldflags=-static" -o kine

FROM scratch
COPY --from=build /kine/kine /bin/kine
CMD ["/bin/kine"]

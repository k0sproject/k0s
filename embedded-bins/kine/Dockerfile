ARG BUILDIMAGE=golang:1.15-alpine
FROM $BUILDIMAGE AS build

ARG VERSION

RUN apk add build-base git


RUN cd / && git clone -b v$VERSION --depth=1 https://github.com/rancher/kine.git
WORKDIR /kine
RUN go build -ldflags="-extldflags=-static -w -s" -o kine

FROM scratch
COPY --from=build /kine/kine /bin/kine
CMD ["/bin/kine"]

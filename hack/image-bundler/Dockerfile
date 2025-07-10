ARG ALPINE_VERSION
FROM docker.io/library/alpine:$ALPINE_VERSION

RUN apk add --no-cache tar containerd containerd-ctr
COPY bundler.sh /
CMD /bundler.sh

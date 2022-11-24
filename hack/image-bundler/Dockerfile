ARG ALPINE_VERSION
FROM docker.io/library/alpine:$ALPINE_VERSION

RUN apk add containerd
COPY bundler.sh /
CMD /bundler.sh

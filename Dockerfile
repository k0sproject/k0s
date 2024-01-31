ARG ARCH
ARG ALPINE_VERSION
FROM docker.io/library/${ARCH}alpine:$ALPINE_VERSION
ARG TARGETARCH

RUN apk add --no-cache iptables tini

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh
ADD ./k0s-${TARGETARCH}/k0s /usr/local/bin/k0s

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh" ]

CMD ["k0s", "controller", "--enable-worker"]

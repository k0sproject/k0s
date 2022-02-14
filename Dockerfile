ARG ARCH
FROM ${ARCH}alpine:3.15
ARG TARGETARCH

RUN apk add --no-cache bash coreutils findutils curl tini

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh
ADD ./k0s-${TARGETARCH} /usr/local/bin/k0s

ENTRYPOINT ["/sbin/tini", "--", "/bin/sh", "/entrypoint.sh" ]


CMD ["k0s", "controller", "--enable-worker"]

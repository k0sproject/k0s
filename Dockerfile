ARG ARCH
ARG ALPINE_VERSION

# Unpack the zip payload that's appended to the k0s executable while building
# the image, so that k0s doesn't have to do it on every container start.
FROM --platform=$BUILDPLATFORM docker.io/library/alpine:$ALPINE_VERSION AS bins
ARG TARGETARCH
RUN apk add --no-cache libcap unzip
ADD ./k0s-${TARGETARCH}/k0s /k0s
RUN mkdir -p /out/bin \
  # unzip exits 1 because of the "extra bytes at beginning" warning that's
  # caused by the k0s executable preceding the appended zip payload.
  && { unzip -oqq /k0s -d /out/bin || [ $? -le 1 ]; } \
  && { [ -n "$(ls -A /out/bin)" ] || { echo 'no zip payload appended to k0s' >&2; exit 1; }; } \
  # The zip entries carry no external attributes, so unzip creates the files
  # with 0600. Use the same permissions that the runtime staging would use.
  && chmod 0750 /out/bin/* && chmod 0755 /k0s \
  # Allow kube-apiserver to bind to privileged ports without being root. This
  # only survives because k0s won't rewrite the pre-unpacked files at runtime.
  && setcap cap_net_bind_service=+ep /out/bin/kube-apiserver \
  # k0s reuses an already staged executable only if its modification time is
  # equal to the one of the k0s executable, compared with nanosecond precision.
  # Stamp both with a whole-second timestamp, so that they compare equal.
  && touch -d "@$(stat -c %Y /k0s)" /k0s /out/bin/*

FROM docker.io/library/${ARCH}alpine:$ALPINE_VERSION

RUN apk add --no-cache iptables tini \
  && for u in etcd kube-apiserver kube-scheduler konnectivity-server; do \
    adduser --system --shell /sbin/nologin --no-create-home --home /var/lib/k0s --disabled-password --gecos '' "$u"; \
  done

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh
# Both of these come from the bins stage, so that the modification times and
# file capabilities established there are preserved.
COPY --from=bins /k0s /usr/local/bin/k0s
COPY --from=bins /out/bin /var/lib/k0s/bin

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh" ]

# Start CMD
CMD ["k0s", "controller", "--enable-worker"]
# End CMD

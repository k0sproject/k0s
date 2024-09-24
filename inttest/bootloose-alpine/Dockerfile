ARG ALPINE_VERSION
ARG GOLANG_IMAGE

FROM docker.io/library/alpine:$ALPINE_VERSION

ARG TARGETARCH
ARG CRI_DOCKERD_VERSION=0.3.15
ARG ETCD_VERSION
ARG TROUBLESHOOT_VERSION=v0.95.1
ARG HELM_VERSION

# Apply our changes to the image
COPY root/ /

RUN apk add --no-cache \
  alpine-base \
  openssh-server \
  bash \
  coreutils \
  curl \
  haproxy \
  nginx \
  inotify-tools \
  ipvsadm

# enable syslog and sshd
RUN rc-update add syslog boot
RUN rc-update add sshd default
RUN rc-update add local default
RUN rc-update add nginx default
# Ensures that /usr/local/bin/k0s is seeded from /dist at startup
RUN rc-update add k0s-seed default
# Prevent start-stop-daemon from hanging when max_fds is huge
RUN sed -Ei -e 's/^[# ](rc_ulimit)=.*/\1="-n 1048576"/' /etc/rc.conf
# disable ttys
RUN sed -i -e 's/^\(tty[0-9]\)/# \1/' /etc/inittab
# enable root logins
RUN sed -i -e 's/^root:!:/root::/' /etc/shadow

# Install troublbeshoot support bundle
RUN curl --proto '=https' --tlsv1.2 -L https://github.com/replicatedhq/troubleshoot/releases/download/$TROUBLESHOOT_VERSION/support-bundle_linux_$TARGETARCH.tar.gz \
  | tar xzO support-bundle >/usr/local/bin/kubectl-supportbundle \
  && chmod +x /usr/local/bin/kubectl-supportbundle

# Put helm into place to ease up debugging and for helm integration tests
RUN curl --proto '=https' --tlsv1.2 -L https://get.helm.sh/helm-v$HELM_VERSION-linux-$TARGETARCH.tar.gz \
  | tar xz linux-$TARGETARCH/helm -C /usr/local/bin --strip-components=1 \
  && chmod +x /usr/local/bin/helm

# Install etcd for smoke tests with external etcd
# No arm binaries available (check-externaletcd won't work on ARMv7)
RUN if [ "$TARGETARCH" != arm ]; then \
    curl --proto '=https' --tlsv1.2 -L https://github.com/etcd-io/etcd/releases/download/v$ETCD_VERSION/etcd-v$ETCD_VERSION-linux-$TARGETARCH.tar.gz \
      | tar xz -C /opt --strip-components=1; \
  fi

# Install cri-dockerd shim for custom CRI testing
# No arm binaries available (check-byocri won't work on ARMv7)
RUN if [ "$TARGETARCH" != arm ]; then \
    curl --proto '=https' --tlsv1.2 --retry 5 --retry-all-errors -sSLfo /tmp/cri-dockerd.tgz https://github.com/Mirantis/cri-dockerd/releases/download/v$CRI_DOCKERD_VERSION/cri-dockerd-$CRI_DOCKERD_VERSION.$TARGETARCH.tgz \
      && tar xf /tmp/cri-dockerd.tgz --directory /tmp/ \
      && mv /tmp/cri-dockerd/cri-dockerd /usr/local/bin/cri-dockerd \
      && rm -rf /tmp/cri-dockerd \
      && chmod 755 /usr/local/bin/cri-dockerd; \
  fi

RUN for u in etcd kube-apiserver kube-scheduler konnectivity-server; do \
    adduser --system --shell /sbin/nologin --no-create-home --home /var/lib/k0s --disabled-password --gecos '' "$u"; \
  done

ADD cri-dockerd.sh /etc/init.d/cri-dockerd

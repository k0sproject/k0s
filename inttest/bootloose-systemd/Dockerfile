ARG UBUNTU_VERSION=24.04
ARG TARGETARCH
ARG ETCD_VERSION
ARG HELM_VERSION
ARG TROUBLESHOOT_VERSION=0.125.0 # renovate: datasource=github-releases depName=replicatedhq/troubleshoot

FROM quay.io/k0sproject/bootloose-ubuntu$UBUNTU_VERSION

ARG TARGETARCH
ARG ETCD_VERSION
ARG HELM_VERSION
ARG TROUBLESHOOT_VERSION

ENV container=docker

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    coreutils \
    curl \
    haproxy \
    inotify-tools \
    iproute2 \
    iptables \
    ipvsadm \
    nginx \
    openssh-server \
    procps \
    systemd \
    systemd-sysv \
  && rm -rf /var/lib/apt/lists/*

COPY root/ /

RUN systemctl enable k0s-seed.service

RUN curl --proto '=https' --tlsv1.2 -L https://github.com/replicatedhq/troubleshoot/releases/download/v$TROUBLESHOOT_VERSION/support-bundle_linux_$TARGETARCH.tar.gz \
  | tar xzO support-bundle >/usr/local/bin/kubectl-supportbundle \
  && chmod +x /usr/local/bin/kubectl-supportbundle

RUN curl --proto '=https' --tlsv1.2 -L https://get.helm.sh/helm-v$HELM_VERSION-linux-$TARGETARCH.tar.gz \
  | tar xzO linux-$TARGETARCH/helm >/usr/local/bin/helm \
  && chmod +x /usr/local/bin/helm

RUN if [ "$TARGETARCH" != "arm" ] && [ "$TARGETARCH" != "riscv64" ]; then \
    curl --proto '=https' --tlsv1.2 -L https://github.com/etcd-io/etcd/releases/download/v$ETCD_VERSION/etcd-v$ETCD_VERSION-linux-$TARGETARCH.tar.gz \
      | tar xz -C /opt --strip-components=1; \
  fi

RUN chmod +x /usr/local/bin/troubleshoot-k0s-inttest.sh

RUN for u in etcd kube-apiserver kube-scheduler konnectivity-server; do \
    useradd --system --shell /usr/sbin/nologin --home-dir /var/lib/k0s --no-create-home "$u"; \
  done

STOPSIGNAL SIGRTMIN+3

CMD ["/lib/systemd/systemd"]

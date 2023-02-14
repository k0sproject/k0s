ARG ALPINE_VERSION
FROM docker.io/library/alpine:$ALPINE_VERSION

ARG ETCD_ARCH
ARG ETCD_VERSION
ARG KUBE_VERSION
ARG HELM_VERSION
# Apply our changes to the image
COPY root/ /

RUN apk add openrc openssh-server bash busybox-openrc coreutils curl haproxy nginx inotify-tools
# enable syslog and sshd
RUN rc-update add cgroups boot
RUN rc-update add syslog boot
RUN rc-update add machine-id boot
RUN rc-update add sshd default
RUN rc-update add local default
RUN rc-update add nginx default
# Ensures that /usr/local/bin/k0s is seeded from /dist at startup
RUN rc-update add k0s-seed default

# remove -docker keyword so we actually mount cgroups in container
RUN sed -i -e '/keyword/s/-docker//' /etc/init.d/cgroups
# disable ttys
RUN sed -i -e 's/^\(tty[0-9]\)/# \1/' /etc/inittab
# enable root logins
RUN sed -i -e 's/^root:!:/root::/' /etc/shadow

# Put kubectl into place to ease up debugging
RUN curl -Lo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v$KUBE_VERSION/bin/linux/amd64/kubectl \
  && chmod +x /usr/local/bin/kubectl
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

# Install troublbeshoot support bundle
RUN curl -Lo - https://github.com/replicatedhq/troubleshoot/releases/download/v0.55.0/support-bundle_linux_amd64.tar.gz \
  | tar xzO support-bundle >/usr/local/bin/kubectl-supportbundle \
  && chmod +x /usr/local/bin/kubectl-supportbundle

# Put helm into place to ease up debugging and for helm integration tests
RUN curl -L https://get.helm.sh/helm-v$HELM_VERSION-linux-amd64.tar.gz \
  | tar xz -C /usr/local/bin --strip-components=1 && chmod +x /usr/local/bin/helm

# Install etcd for smoke tests with external etcd
RUN curl -L https://github.com/etcd-io/etcd/releases/download/v$ETCD_VERSION/etcd-v$ETCD_VERSION-linux-$ETCD_ARCH.tar.gz \
  | tar xz -C /opt --strip-components=1

# Install cri-dockerd shim for custom CRI testing
RUN curl -sSfLo /tmp/cri-dockerd.tgz https://github.com/Mirantis/cri-dockerd/releases/download/v0.3.1/cri-dockerd-0.3.1.amd64.tgz \
  && tar xf /tmp/cri-dockerd.tgz --directory /tmp/ \
  && mv /tmp/cri-dockerd/cri-dockerd /usr/local/bin/cri-dockerd \
  && rm -rf /tmp/cri-dockerd \
  && chmod 755 /usr/local/bin/cri-dockerd
ADD cri-dockerd.sh /etc/init.d/cri-dockerd

FROM alpine:3.13

ENV K0S_VERSION_URL=https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.0/k0s-v1.23.3+k0s.0-amd64
ENV KUBE_VERSION=1.23.3

RUN apk add openrc openssh-server bash busybox-initscripts coreutils findutils curl haproxy
# enable syslog and sshd
RUN rc-update add cgroups boot
RUN rc-update add syslog boot
RUN rc-update add machine-id boot
RUN rc-update add sshd default
RUN rc-update add local default

# remove -docker keyword so we actually mount cgroups in container
RUN sed -i -e '/keyword/s/-docker//' /etc/init.d/cgroups

# disable ttys
RUN sed -i -e 's/^\(tty[0-9]\)/# \1/' /etc/inittab

# enable root logins
RUN sed -i -e 's/^root:!:/root::/' /etc/shadow

# Install k0s as a service, but don't add it to a runlevel (don't start up)
RUN curl -L ${K0S_VERSION_URL} -o /usr/local/bin/k0s && chmod +x /usr/local/bin/k0s
RUN /usr/local/bin/k0s install controller && rc-update del k0scontroller
RUN /usr/local/bin/k0s install worker && rc-update del k0sworker

# Put kubectl into place to ease up debugging
RUN curl -L https://storage.googleapis.com/kubernetes-release/release/v$KUBE_VERSION/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
       && chmod +x /usr/local/bin/kubectl

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf
# This lets etcd start when running on arm in smokes
ENV ETCD_UNSUPPORTED_ARCH=arm64

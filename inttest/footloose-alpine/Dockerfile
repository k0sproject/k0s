FROM alpine:3.13

RUN apk add openrc openssh-server bash busybox-initscripts coreutils findutils iptables curl
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

# Put kubectl into place to ease up debugging
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubectl \
       && chmod +x ./kubectl \
       && mv ./kubectl /usr/local/bin/kubectl
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf
# This lets etcd start when running on arm in smokes
ENV ETCD_UNSUPPORTED_ARCH=arm64

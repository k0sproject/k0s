FROM alpine:3.12

RUN apk add openrc openssh-server bash busybox-initscripts coreutils findutils iptables curl
# enable syslog and sshd
RUN rc-update add cgroups boot
RUN rc-update add syslog boot
RUN rc-update add sshd default
RUN rc-update add local default
# remove -docker keyword so we actually mount cgroups in container
RUN sed -i -e '/keyword/s/-docker//' /etc/init.d/cgroups
# disable ttys
RUN sed -i -e 's/^\(tty[0-9]\)/# \1/' /etc/inittab
# enable root logins
RUN sed -i -e 's/^root:!:/root::/' /etc/shadow
RUN echo "#!/bin/sh" > /etc/local.d/machine-id.start \
       && echo "if ! [ -f /etc/machine-id ]; then" >> /etc/local.d/machine-id.start \
       && echo "  dd if=/dev/urandom status=none bs=16 count=1 | md5sum | cut -d' ' -f1 > /etc/machine-id" >> /etc/local.d/machine-id.start \
       && echo "fi" >> /etc/local.d/machine-id.start \
       && chmod +x /etc/local.d/machine-id.start

# Put kubectl into place to ease up debugging
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.19.0/bin/linux/amd64/kubectl \
       && chmod +x ./kubectl \
       && mv ./kubectl /usr/local/bin/kubectl
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

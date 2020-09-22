FROM alpine:3.12

RUN apk add openrc openssh-server bash busybox-initscripts coreutils findutils iptables
# enable syslog and sshd
RUN rc-update add cgroups boot
RUN rc-update add syslog boot
RUN rc-update add sshd default
# remove -docker keyword so we actually mount cgroups in container
RUN sed -i -e '/keyword/s/-docker//' /etc/init.d/cgroups
# disable ttys
RUN sed -i -e 's/^\(tty[0-9]\)/# \1/' /etc/inittab
# enable root logins
RUN sed -i -e 's/^root:!:/root::/' /etc/shadow


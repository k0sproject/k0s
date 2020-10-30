FROM alpine:3.12
RUN dd if=/dev/urandom status=none bs=16 count=1 | md5sum | cut -d' ' -f1 > /etc/machine-id
# RUN echo "nameserver 8.8.8.8" > /etc/resolv.conf

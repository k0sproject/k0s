FROM alpine:3.12
RUN dd if=/dev/urandom status=none bs=16 count=1 | md5sum | cut -d' ' -f1 > /etc/machine-id
ADD ./fixresolv.sh /fixresolv.sh
ADD ./mke /usr/local/bin/mke
CMD [ "/bin/sh", "/fixresolv.sh" ]
# CMD ["/usr/locla/bin/mke", "server", "--enable-worker"]
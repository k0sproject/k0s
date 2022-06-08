FROM alpine:3.16

RUN apk add containerd

ADD bundler.sh /bundler.sh
ADD image.list /image.list

CMD /bundler.sh

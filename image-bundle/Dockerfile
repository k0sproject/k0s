FROM alpine:3.13

RUN apk add containerd

ADD bundler.sh /bundler.sh
ADD image.list /image.list

CMD /bundler.sh
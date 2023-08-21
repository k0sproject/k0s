ARG BASE
FROM ${BASE}

RUN apk add nginx

RUN rc-update add nginx boot && mkdir -p /run/nginx/

ADD html /var/lib/nginx/html
ADD nginx.conf /etc/nginx/nginx.conf
ADD default.conf /etc/nginx/http.d/default.conf

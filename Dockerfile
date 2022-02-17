FROM alpine:3.13

RUN apk add --no-cache bash coreutils findutils iptables curl tini

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.20.1/bin/linux/amd64/kubectl \
       && chmod +x ./kubectl \
       && mv ./kubectl /usr/local/bin/kubectl
ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

ADD docker-entrypoint.sh /entrypoint.sh
ADD ./k0s /usr/local/bin/k0s 

ENTRYPOINT ["/sbin/tini", "--", "/bin/sh", "/entrypoint.sh" ]


CMD ["k0s", "controller", "--enable-worker"]

FROM quay.io/footloose/ubuntu18.04

ADD mke.service /etc/systemd/system/mke.service

RUN curl -L -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.19.0/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl

ENV KUBECONFIG=/var/lib/mke/pki/admin.conf

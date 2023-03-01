FROM quay.io/footloose/ubuntu18.04

ADD k0s.service /etc/systemd/system/k0s.service

RUN curl -L -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.26.2/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

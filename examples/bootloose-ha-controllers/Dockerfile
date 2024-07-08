FROM quay.io/k0sproject/bootloose-ubuntu18.04

ADD k0s.service /etc/systemd/system/k0s.service

RUN curl --proto '=https' --tlsv1.2 --retry 5 --retry-all-errors -Lfo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.30.2/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl

ENV KUBECONFIG=/var/lib/k0s/pki/admin.conf

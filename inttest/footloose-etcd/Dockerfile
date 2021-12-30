FROM quay.io/footloose/ubuntu18.04

RUN apt-get -y update
RUN apt-get install -y software-properties-common
RUN add-apt-repository universe
RUN apt install -y etcd etcd-client

RUN mkdir -p /etc/systemd/system/etcd.service.d
ADD etcd.conf /etc/systemd/system/etcd.service.d
RUN systemctl enable etcd

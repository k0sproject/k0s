FROM quay.io/footloose/ubuntu18.04

RUN apt-get update && apt-get install -y vim && \
    addgroup mke &&\
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ADD mke.yaml /root/mke.yaml

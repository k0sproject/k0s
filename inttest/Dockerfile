FROM quay.io/footloose/ubuntu18.04

RUN curl -L --silent https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_linux_amd64.tar.gz | tar -C bin/ -zxv sonobuoy && \
    mv bin/sonobuoy /usr/bin/sonobuoy
 

ADD mke /usr/bin/mke


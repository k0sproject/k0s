FROM golang:1.15-alpine AS build

ARG VERSION
ENV GOPATH=/go
ENV COMMANDS="kubelet kube-apiserver kube-scheduler kube-controller-manager"

RUN apk add build-base git go-bindata linux-headers rsync grep coreutils bash

RUN mkdir -p $GOPATH/src/github.com/kubernetes/kubernetes
RUN git clone -b v$VERSION --depth=1 https://github.com/kubernetes/kubernetes.git $GOPATH/src/github.com/kubernetes/kubernetes
WORKDIR /go/src/github.com/kubernetes/kubernetes
RUN \
	for cmd in $COMMANDS; do \
		export KUBE_GIT_VERSION="v$VERSION-k0s1"; \
		make GOFLAGS="-v -tags=providerless" GOLDFLAGS="-extldflags=-static -w -s" WHAT=cmd/$cmd || break;\
	done

FROM scratch
COPY --from=build \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kubelet \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kube-apiserver \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kube-scheduler \
	/go/src/github.com/kubernetes/kubernetes/_output/local/bin/linux/*/kube-controller-manager \
	/bin/
CMD ["/bin/kubelet"]

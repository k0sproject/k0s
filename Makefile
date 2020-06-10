
bin:
	mkdir -p bin/

bin/runc:
	curl -L -o bin/runc https://github.com/opencontainers/runc/releases/download/v1.0.0-rc90/runc.amd64

bin/containerd: bin
	curl -L https://github.com/containerd/containerd/releases/download/v1.3.4/containerd-1.3.4.linux-amd64.tar.gz | tar zxvf -

bin/kubelet: bin
	curl -L -o bin/kubelet https://storage.googleapis.com/kubernetes-release/release/v1.18.2/bin/linux/amd64/kubelet

pkg/assets/zz_generated_bindata.go: bin/kubelet bin/containerd bin/runc
	go-bindata -o pkg/assets/zz_generated_bindata.go -pkg assets bin/

build: pkg/assets/zz_generated_bindata.go
	go build -o mke main.go

clean:
	rm -f pkg/assets/zz_generated_bindata.go mke
	rm -rf bin/

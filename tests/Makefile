
footloose_version = 0.6.3
kubectl_version = 1.19.0

footloose_url = https://github.com/weaveworks/footloose/releases/download/$(footloose_version)/footloose-$(footloose_version)-linux-x86_64
kubectl_url = https://storage.googleapis.com/kubernetes-release/release/v$(kubectl_version)/bin/linux/amd64/kubectl
sonobuoy_url = https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_linux_amd64.tar.gz

curl = curl -L --silent

bins = bin/footloose bin/kubectl bin/sonobuoy

.PHONY: all
all: $(bins) id_ed25519_k0s

bin:
	mkdir -p $@

bin/footloose bin/kubectl: | bin
	$(curl) -o $@.tmp $($(notdir $@)_url)
	chmod +x $@.tmp && mv $@.tmp $@

bin/sonobuoy:
	$(curl) $(sonobuoy_url) | tar -C bin/ -zxv $(notdir $@)

id_ed25519_k0s:
	ssh-keygen -t ed25519 -N "" -f $@


.test-%: %.bash footloose-%.yaml.tpl $(bins) id_ed25519_k0s ../k0s
	bash $<
	touch $@

.PHONY: check
check: $(bins) .test-sig-network

.PHONY: clean
clean:
	rm -rf bin id_ed25519_k0s* footloose-*.yaml token kubeconfig *_sonobuoy_*

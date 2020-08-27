
footloose_version = 0.6.3
kubectl_version = 1.18.6

footloose_url = https://github.com/weaveworks/footloose/releases/download/$(footloose_version)/footloose-$(footloose_version)-linux-x86_64
kubectl_url = https://storage.googleapis.com/kubernetes-release/release/v$(kubectl_version)/bin/linux/amd64/kubectl

curl = curl -L --silent

bins = bin/footloose bin/kubectl

.PHONY: all
all: $(bins) id_ed25519_mke

bin:
	mkdir -p $@

bin/footloose bin/kubectl: | bin
	$(curl) -o $@.tmp $($(notdir $@)_url)
	chmod +x $@.tmp && mv $@.tmp $@

id_ed25519_mke:
	ssh-keygen -t ed25519 -N "" -f $@


.test-%: %.bats footloose-%.yaml.tpl $(bins) id_ed25519_mke ../mke
	bats $<
	touch $@

.PHONY: check
check: $(bins) .test-single

.PHONY: clean
clean:
	rm -rf bin id_ed25519_mke* footloose-*.yaml

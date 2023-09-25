# Install using custom CA certificates and SA key pair

k0s generates all needed certificates automatically in the `<data-dir>/pki` directory (`/var/lib/k0s/pki`, by default).  

But sometimes there is a need to have the CA certificates and SA key pair in advance.
To make it work, just put files to the `<data-dir>/pki` and `<data-dir>/pki/etcd`:

```shell
export LIFETIME=365
mkdir -p /var/lib/k0s/pki/etcd
cd /var/lib/k0s/pki
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days $LIFETIME -out ca.crt -subj "/CN=Custom CA"
openssl genrsa -out sa.key 2048
openssl rsa -in sa.key -outform PEM -pubout -out sa.pub
cd ./etcd
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days $LIFETIME -out ca.crt -subj "/CN=Custom CA"
```

Then you can [install k0s as usual](./install.md).

## Pre-generated tokens

It's possible to get join in advance without having a running cluster.

```shell
k0s token pre-shared --role worker --cert /var/lib/k0s/pki/ca.crt --url https://<controller-ip>:6443/
```

The command above generates a join token and a Secret. A Secret should be deployed to the cluster to authorize the token.
For example, you can put the Secret under the [manifest](manifests.md) directory and it will be deployed automatically.

Please note that if you are generating a join token for a controller, the port number needs to be 9443 instead of 6443.
Controller bootstrapping requires talking to the k0s-apiserver instead of the kube-apiserver.
Here's an example of a command for pre-generating a token for a controller.

```shell
k0s token pre-shared --role controller --cert /var/lib/k0s/pki/ca.crt --url https://<controller-ip>:9443/
```

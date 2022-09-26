# Install using custom CA certificate

k0s generates all needed certificates automatically in the `<data-dir>/pki` directory (`/var/lib/k0s/pki`, by default).  

But sometimes there is a need to have the CA certificate in advance.
To make it work, just put `ca.key` and `ca.crt` files to the `<data-dir>/pki`:

```shell
mkdir -p /var/lib/k0s/pki
cd /var/lib/k0s/pki
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 -out ca.crt -subj "/CN=Custom CA"
```

Then you can [install k0s as usual](./install.md).

## Pre-generated tokens

It's possible to get join in advance without having a running cluster.

```shell
k0s token pre-shared --role worker --cert /var/lib/k0s/pki/ca.crt --url https://<controller-ip>:6443/
```

The command above generates a join token and a Secret. A Secret should be deployed to the cluster to authorize the token.
For example, you can put the Secret under the [manifest](manifests.md) directory and it will be deployed automatically.
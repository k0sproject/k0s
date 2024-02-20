# Verifying Signed Binaries

K0smotron team provides signed binaries for k0s. The signatures are created using [cosign](https://docs.sigstore.dev/signing/quickstart/).
Public key and signature files are available for download from the [releases page](https://github.com/k0sproject/k0s/releases/latest).
Binaries can be verified using the `cosign` tool, for example:

```shell
cosign verify-blob \
  --key https://github.com/k0sproject/k0s/releases/download/v{{{ extra.k8s_version }}}%2Bk0s.0/cosign.pub \
  --signature https://github.com/k0sproject/k0s/releases/download/v{{{ extra.k8s_version }}}%2Bk0s.0/k0s-v{{{ extra.k8s_version }}}+k0s.0-amd64.sig \
  --payload k0s-v{{{ extra.k8s_version }}}+k0s.0-amd64
```

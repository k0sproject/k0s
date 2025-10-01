<!--
SPDX-FileCopyrightText: 2023 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Verifying Signed Binaries

K0smotron team provides signed binaries for k0s. The signatures are created using [cosign](https://docs.sigstore.dev/signing/quickstart/).
Public key and signature files are available for download from the [releases page](https://github.com/k0sproject/k0s/releases/latest).
Binaries can be verified using the `cosign` tool, for example:

{% set k0s_url_ver = k0s_version | urlencode -%}

```shell
cosign verify-blob \
  --key https://github.com/k0sproject/k0s/releases/download/{{{ k0s_url_ver }}}/cosign.pub \
  --signature https://github.com/k0sproject/k0s/releases/download/{{{ k0s_url_ver }}}/k0s-{{{ k0s_url_ver }}}-amd64.sig \
  k0s-{{{ k0s_version }}}-amd64
```

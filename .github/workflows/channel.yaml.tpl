version: "{{ .Version }}"
downloadURLs:
  k0s:
    linux-amd64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-{{ .Version }}-amd64"
    linux-arm: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-{{ .Version }}-arm"
    linux-arm64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-{{ .Version }}-arm64"
  airgap:
    linux-airgap-bundle-amd64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-airgap-bundle-{{ .Version }}-amd64"
    linux-airgap-bundle-arm: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-airgap-bundle-{{ .Version }}-arm"
    linux-airgap-bundle-arm64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-airgap-bundle-{{ .Version }}-arm64"

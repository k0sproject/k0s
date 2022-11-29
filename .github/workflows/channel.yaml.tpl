version: "{{ .Version }}"
downloadURLs:
  k0s:
    k0s-amd64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-{{ .Version }}-amd64"
    k0s-arm: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-{{ .Version }}-arm"
    k0s-arm64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-{{ .Version }}-arm64"
  airgap:
    k0s-airgap-bundle-amd64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-airgap-bundle-{{ .Version }}-amd64"
    k0s-airgap-bundle-arm: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-airgap-bundle-{{ .Version }}-arm"
    k0s-airgap-bundle-arm64: "https://github.com/k0sproject/k0s/releases/download/{{ .Version }}/k0s-airgap-bundle-{{ .Version }}-arm64"

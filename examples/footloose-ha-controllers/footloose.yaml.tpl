cluster:
  name: mke
  privateKey: ~/.ssh/id_rsa
machines:
- count: 3
  backend: docker
  spec:
    image: quay.io/footloose/ubuntu18.04
    name: controller%d
    privileged: true
    volumes:
    - type: bind
      source: /lib/modules
      destination: /lib/modules
    - type: bind
      source: $PWD/../../mke
      destination: /usr/local/bin/mke
    - type: bind
      source: $PWD/mke.yaml
      destination: /etc/mke/config.yaml
    - type: bind
      source: $PWD/mke.service
      destination: /etc/systemd/system/mke.service
    - type: volume
      destination: /var/lib/containerd
    - type: volume
      destination: /var/lib/kubelet
    portMappings:
    - containerPort: 22
      hostPort: 9222
    - containerPort: 6443
      hostPort: 6443
    - containerPort: 8080
      hostPort: 8080

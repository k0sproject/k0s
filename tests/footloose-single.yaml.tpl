cluster:
  name: $CLUSTER_NAME
  privateKey: id_ed25519_mke
machines:
- count: 1
  backend: docker
  spec:
    image: $LINUX_IMAGE
    name: node%d
    privileged: true
    volumes:
    - type: bind
      source: $MKE_BINARY
      destination: /usr/bin/mke
    - type: volume
      destination: /var/lib/containerd
    - type: volume
      destination: /var/lib/kubelet
    networks:
    - $NETWORK_NAME
    portMappings:
    - containerPort: 22
      hostPort: 9022


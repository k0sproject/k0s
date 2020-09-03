cluster:
  name: $CLUSTER_NAME
  privateKey: id_ed25519_mke
machines:
- count: 3
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
      destination: /var/lib/mke
    portMappings:
    - containerPort: 22
      hostPort: 9022
    - containerPort: 6443
      hostPort: 6443


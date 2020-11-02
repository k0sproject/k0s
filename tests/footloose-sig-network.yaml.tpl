cluster:
  name: $CLUSTER_NAME
  privateKey: id_ed25519_k0s
machines:
- count: 3
  backend: docker
  spec:
    image: $LINUX_IMAGE
    name: node%d
    privileged: true
    volumes:
    - type: bind
      source: $K0S_BINARY
      destination: /usr/bin/k0s
    - type: bind
      source: /lib/modules
      destination: /lib/modules
    - type: volume
      destination: /var/lib/k0s
    portMappings:
    - containerPort: 22
      hostPort: 9022
    - containerPort: 6443
      hostPort: 6443


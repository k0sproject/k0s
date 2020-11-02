cluster:
  name: k0s
  privateKey: ~/.ssh/id_rsa
machines:
- count: 3
  backend: docker
  spec:
    image: k0s-footloose:latest
    name: controller%d
    privileged: true
    volumes:
    - type: bind
      source: /lib/modules
      destination: /lib/modules
    - type: bind
      source: $PWD/../../k0s
      destination: /usr/local/bin/k0s
    - type: bind
      source: $PWD/k0s.yaml
      destination: /etc/k0s/config.yaml
    - type: volume
      destination: /var/lib/k0s
    portMappings:
    - containerPort: 22
      hostPort: 9222
    - containerPort: 6443
      hostPort: 6443
    - containerPort: 8080
      hostPort: 8080

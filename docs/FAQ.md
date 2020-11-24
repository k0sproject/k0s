# Frequently asked questions

### How is k0s pronounced?

kay-zero-ess

### How do I run a single node cluster?

`k0s server --enable-worker`

### How do I connect to the cluster?

You find the config in `/var/lib/k0s/pki/admin.conf`. Copy this and
change the `localhost` to the public ip of the controller. Use the
modified config to connect with kubectl:
```
export KUBECONFIG=/path/to/admin.conf
kubectl ...
```


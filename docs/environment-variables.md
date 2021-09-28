# Environment variables

`k0s install` does not support environment variables.

Setting environment variables for components used by k0s depends on the used init system. The environment variables set in `k0scontroller` or `k0sworker` service will be inherited by k0s components, such as `etcd`, `containerd`, `konnectivity`, etc.

Component specific environment variables can be set in `k0scontroller` or `k0sworker` service. For example: for `CONTAINERD_HTTPS_PROXY`, the prefix `CONTAINERD_` will be stripped and converted to `HTTPS_PROXY` in the `containerd` process.

For those components having env prefix convention such as `ETCD_xxx`, they are handled specially, i.e. the prefix will not be stripped. For example, `ETCD_MAX_WALS` will still be `ETCD_MAX_WALS` in etcd process.

The proxy envs `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` are always overridden by component specific environment variables, so `ETCD_HTTPS_PROXY` will still be converted to `HTTPS_PROXY` in etcd process.

## SystemD

Create a drop-in directory and add config file with a desired environment variable:

```shell
mkdir -p /etc/systemd/system/k0scontroller.service.d
tee -a /etc/systemd/system/k0scontroller.service.d/http-proxy.conf <<EOT
[Service]
Environment=HTTP_PROXY=192.168.33.10:3128
EOT
```

## OpenRC

Export desired environment variable overriding service configuration in /etc/conf.d directory:

```shell
echo 'export HTTP_PROXY="192.168.33.10:3128"' > /etc/conf.d/k0scontroller
```

# Environment variables

`k0s install` does not support environment variables.

Setting environment variables for components used by k0s depends on the used init system. The environment variables set in `k0scontroller` or `k0sworker` service will be inherited by k0s components, such as `etcd`, `containerd`, etc.

Component specific environment variables can be set in `k0scontroller` or `k0sworker` service. For example: `CONTAINERD_HTTPS_PROXY` will be converted to `HTTPS_PROXY` in the `containerd` process while other components are not affected.

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

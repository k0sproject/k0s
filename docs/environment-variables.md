# Environment variables

`k0s install` does not support environment variables.
Setting environment variable for a component used by k0s depends on used init system.

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

# Configuration validation

k0s command-line interface has the ability to validate config syntax:

```shell
k0s validate config --config /etc/k0s/k0s.yaml
```

`validate config` sub-command can validate the following:

1. YAML formatting
2. [SAN addresses](/configuration/#specapi)
3. [Network providers](/configuration/#specnetwork)
4. [Worker profiles](/configuration/#specworkerprofiles)

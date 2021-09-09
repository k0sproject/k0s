# Configuration validation

k0s command-line interface has the ability to validate config syntax:

```shell
k0s validate config --config path/to/config/file
```

`validate config` sub-command can validate the following:

1. YAML formatting
2. [SAN addresses](/main/configuration/#specapi)
3. [Network providers](/main/configuration/#specnetwork)
4. [Worker profiles](/main/configuration/#specworkerprofiles)

# Configuration validation

k0s command-line interface has the ability to validate config syntax:

```shell
k0s config validate --config path/to/config/file
```

`config validate` sub-command can validate the following:

1. YAML formatting
2. [SAN addresses](configuration.md#specapi)
3. [Network providers](configuration.md#specnetwork)
4. [Worker profiles](configuration.md#specworkerprofiles)

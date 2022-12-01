# kcli plans for k0s

This tool allows deploying clusters on libvirt using [kcli](https://github.com/karmab/kcli).
Kcli allows to create VMs, among other things, in libvirt. By defining plans you can lunch
full fledged clusters in a single command.

This has three advantages over running loads on AWS:
1- It doesn't involve additional costs
2- Given that you have appropriate hardware, it's faster to deploy it this way.
3- It's very unlikely that you forget running instances.

## Security considerations

**IMPORTANT!!**: Your private key is copied into the virtual machines so that they can gather the k0s binaries.
When using a local hypervisor this is fairly safe as long as you don't share a snapshot with anyone. But keep
in mind the possible security implications of giving access to this vm.

## Requirements

This uses relative paths in a very fragile way. You are expected to be in the same directory as
the plan name.

You are expected to have:

- [kcli](https://github.com/karmab/kcli) up and running. Including all its dependencies.
- the environment variable GOPATH defined.
- k0s binary compiled in `../../k0s` (can be overriden with the parameter k0spath)
- k0sctl binary compiled in `../../../k0sctl/k0sctl` (can be overriden with the parameter k0sctlpath)

## Examples

### Creating an all in one cluster

You can create a plan with default values simply by running the command:

```bash
kcli create plan -f aio.yml
```

### Custom parameters

The plan ships sane defaults, but if you need to do some customization such custom cpu number, you
can use the -P flag. For instance to change the memory use -P memory=8192. Every plan has its parameters
defined in the plan definiton in the section `.paramters`. For instance to launch an all in one cluster
with custom memory allocation you would run:

```bash
kcli create plan -f aio.yml -P memory=8192
```

### Deleting a plan

Find the plan using:

```bash
kcli list plan
```

And delete it using:

```bash
kcli remove plan <plan name>
```

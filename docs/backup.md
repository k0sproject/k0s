# Backup/Restore overview

k0s has integrated support for backing up cluster state and configuration.
The k0s backup utility is aiming to back up and restore k0s managed parts of the cluster.

The backups created by `k0s backup` command have following pieces of your cluster:

- certificates (the content of the `<data-dir>/pki` directory)
- etcd snapshot, if the etcd storage is used
- k0s.yaml
- any custom defined manifests under the `<data-dir>/manifests`
- any image bundles located under the `<data-dir>/images`
- any helm configuration

Parts **NOT** covered by the backup utility:

- PersistentVolumes of any running application
- database content, in case if the `kine` is used as a storage driver
- any configuration to the cluster introduced by manual changes (e.g. changes that weren't saved under the `<data-dir>/manifests`)

Any of the backup/restore related operations MUST be performed on the controller node.

## Backup

To create backup run the following command on the controller node:

```shell
k0s backup --save-path=<directory>
```

The directory used for the `save-path` value must exist and be writable. The default value is the current working directory.
The command provides backup archive using following naming convention: `k0s_backup_<ISODatetimeString>.tar.gz`

Because of the DateTime usage, it is guaranteed that none of the previously created archives would be overwritten.

## Restore

To restore cluster state from the archive use the following command on the controller node:

```shell
k0s restore /tmp/k0s_backup_2021-04-26T19_51_57_000Z.tar.gz
```

The command would fail if the data directory for the current controller has overlapping data with the backup archive content.
The command would use the archived `k0s.yaml` as the cluster configuration description.

In case if your cluster is HA, after restoring single controller node, join the rest of the controller nodes to the cluster.
E.g. steps for N nodes cluster would be:

- Restore backup on fresh machine
- Run controller there
- Join N-1 new machines to the cluster the same way as for the first setup.

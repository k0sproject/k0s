<!--
SPDX-FileCopyrightText: 2026 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# k0s directories

This page describes the directories k0s reads and writes on the host, what they
are used for, and the permissions k0s expects on them.

**Please note that the directory layouts described here are considered an
implementation detail and may change without further notice, even between patch
releases, unless explicitly stated otherwise, e.g., by other parts of the
documentation referencing those paths or files within them.**

The two parent directories are configurable via CLI flags:

- `--data-dir` (default `/var/lib/k0s` on Linux, `C:\var\lib\k0s` on Windows)<br>
  persistent state
- `--run-dir` (default `/run/k0s` on Linux)<br>
  runtime state such as PID files and unix sockets

Additionally, on k0s nodes with workloads enabled, you can set the following directories:

- `--kubelet-root-dir` (default `<data-dir>/kubelet`)<br>
  Kubelet root directory

## Data directory layout

Inside `<data-dir>` (default `/var/lib/k0s`):

| Path                           | Purpose                                                                                | Mode   |
| ------------------------------ | -------------------------------------------------------------------------------------- | ------ |
| `<data-dir>/`                  | Top-level data directory.                                                              | `0755` |
| `<data-dir>/bin/`              | Embedded binaries (kubelet, etcd, containerd, …) extracted on first start.             | `0755` |
| `<data-dir>/pki/`              | Cluster CA and component certificates managed by k0s.                                  | `0751` |
| `<data-dir>/pki/etcd/`         | etcd-specific certificates.                                                            | `0711` |
| `<data-dir>/etcd/`             | etcd database files if k0s is configured to use the embedded etcd.                     | `0700` |
| `<data-dir>/manifests/`        | Manifest Deployer drop-in directory. Files placed here are applied to the cluster.     | `0755` |
| `<data-dir>/images/`           | Image bundles imported into the managed containerd instance                            | `0755` |
| `<data-dir>/kine/`             | Kine database files when k0s is configured to use Kine instead of etcd.                | `0750` |

Individual certificate files inside `pki/` are written with mode `0644`
(public certs) or `0640` (private keys), and admin/kubelet config files are
written with mode `0600`.

## Run directory layout

Inside `<run-dir>` (default `/run/k0s`):

| Path                            | Purpose                                                                           | Mode            |
| ------------------------------- | --------------------------------------------------------------------------------- | --------------- |
| `<run-dir>/`                    | Top-level run directory.                                                          | `0755`          |
| `<run-dir>/*.pid`               | PID files for supervised processes (kubelet, etcd, containerd, …).                | `0644`          |
| `<run-dir>/status.sock`         | Unix socket exposing local status to `k0s status` and similar commands.           | (process umask) |
| `<run-dir>/etcd/`               | Directory of the embedded etcd's client socket, owned by the etcd user.           | `0755`          |
| `<run-dir>/etcd/localhost:2379` | TLS unix socket of the embedded etcd. k0s components connect to etcd through it.¹ | `0666`          |

¹ This path is stable, so tools on the node can connect to etcd at
`unixs://<run-dir>/etcd/localhost:2379` with the etcd client certificates. The
name is not a TCP address. See [etcd client
URLs](configuration.md#specstorageetcdextraargs-client-urls).

The run directory is intentionally outside the data directory so that on
systems where `/run` is a tmpfs, runtime state is cleared on reboot while
persistent state under `<data-dir>` survives.

## Notes

- All paths above are derived from the same `--data-dir` / `--run-dir` flags
  passed to `k0s` and propagated to its supervised components, so changing
  the flags moves the whole tree consistently.
- The CSI guide in the [Storage](storage.md) section documents the kubelet root
  path caveat, since the default differs from upstream Kubernetes
  (`<data-dir>/kubelet` rather than `/var/lib/kubelet`).
- Backup/restore covers `pki/`, `manifests/`, and `images/`, along with the etcd
  or k0s database files, ik k0s is configured to use them. See
  [Backup/Restore](backup.md) for details.

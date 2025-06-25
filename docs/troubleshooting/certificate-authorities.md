<!--
SPDX-FileCopyrightText: 2024 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Certificate Authorities (CAs)

## Overview of CAs managed by k0s

k0s maintains two Certificate Authorities and one public/private key pair:

* The **Kubernetes CA** is used to secure the Kubernetes cluster and manage
  client and server certificates for API communication.
* The **etcd CA** is used only when managed etcd is enabled, for securing etcd
  communications.
* The **Kubernetes Service Account (SA) key pair** is used for signing
  Kubernetes [service account tokens].

These CAs are automatically created during cluster initialization and have a
default expiration period of 10 years. They are distributed once to all k0s
controllers as part of k0s's [join process]. Replacing them is a manual process,
as k0s currently lacks automation for CA renewal.

[service account tokens]: https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/
[join process]: ../k0s-multi-node.md#5-add-controllers-to-the-cluster

## Replacing the Kubernetes CA and SA key pair

The following steps describe a way how to manually replace the Kubernetes CA and
SA key pair by taking a cluster down, regenerating those and redistributing them
to all nodes, and then bringing the cluster back online:

1. Take a [backup]! Things might go wrong at any level.

2. Stop k0s on all worker and controller nodes. All the instructions below
   assume that all k0s nodes are using the default data directory
   `/var/lib/k0s`. Please adjust accordingly if you're using a different data
   directory path.

3. Delete the Kubernetes CA and SA key pair files from the all the controller
   data directories:

   * `/var/lib/k0s/pki/ca.crt`
   * `/var/lib/k0s/pki/ca.key`
   * `/var/lib/k0s/pki/sa.pub`
   * `/var/lib/k0s/pki/sa.key`

   Delete the kubelet's kubeconfig file and the kubelet's PKI directory from all
   worker data directories. Note that this includes controllers that have been
   started with the `--enable-worker` flag:

   * `/var/lib/k0s/kubelet.conf`
   * `/var/lib/k0s/kubelet/pki`

4. Choose one controller as the "first" one. Restart k0s on the first
   controller. If this controller is running with the `--enable-worker` flag,
   you should **reboot the machine** instead. This will ensure that all
   processes and pods will be cleanly restarted. After the restart, k0s will
   have regenerated a new Kubernetes CA and SA key pair.

5. Distribute the new CA and SA key pair to the other controllers: Copy over the
   following files from the first controller to each of the remaining
   controllers:

   * `/var/lib/k0s/pki/ca.crt`
   * `/var/lib/k0s/pki/ca.key`
   * `/var/lib/k0s/pki/sa.pub`
   * `/var/lib/k0s/pki/sa.key`

   After copying the files, the new CA and SA key pair are in place. Restart k0s
   on the other controllers. For controllers running with the `--enable-worker`
   flag, **reboot the machines** instead.

6. Rejoin all workers. The easiest way to do this is to use a
   `kubelet-bootstrap.conf` file. You can [generate](../cli/k0s_token_create.md)
   such a file on a controller like this (see the section on [join tokens] for
   details):

   ```sh
   touch /tmp/rejoin-token &&
     chmod 0600 /tmp/rejoin-token &&
     k0s token create --expiry 1h |
     base64 -d |
     gunzip >/tmp/rejoin-token
   ```

   Copy that token to each worker node and place it at
   `/var/lib/k0s/kubelet-bootstrap.conf`. Then reboot the machine.

7. When all workers are back online, the `kubelet-bootstrap.conf` files can be
   safely removed from the workers. You can also invalidate the token so you
   don't have to wait for it to expire: Use [`k0s token list --role
   worker`](../cli/k0s_token_list.md) to list all tokens and [`k0s token
   invalidate <token-id>`](../cli/k0s_token_invalidate.md) to invalidate them immediately.

[backup]: ../backup.md
[join tokens]: ../k0s-multi-node.md#about-join-tokens

## See also

* [Install using custom CAs](../custom-ca.md)

# Creating a cluster with Ansible Playbook

Using Ansible and the k0s-ansible playbook, you can install a multi-node Kubernetes Cluster in a couple of minutes. Ansible is a popular infrastructure as code tool which helps you automate tasks to achieve the desired state in a system.

This guide shows how you can install k0s on local virtual machines. In this guide, the following tools are used:

- `multipass`, a lightweight VM manager that uses KVM on Linux, Hyper-V on Windows, and hypervisor.framework on macOS ([installation guide](https://multipass.run/docs)).
- `ansible`, a popular infrastructure as code tool ([installation guide](https://docs.ansible.com/ansible/latest/installation_guide/index.html)).
- and of course `kubectl` on your local machine ([Installation guide](https://kubernetes.io/docs/tasks/tools/install-kubectl/)).

Before following this tutorial, you should have a general understanding of Ansible. A great way to start is the official [Ansible User Guide](https://docs.ansible.com/ansible/latest/user_guide/index.html).

_Please note: k0s users created k0s-ansible. Please send your feedback, bug reports, and pull requests to [github.com/movd/k0s-ansible](https://github.com/movd/k0s-ansible)._

Without further ado, let's jump right in.

## Download k0s-ansible

On your local machine clone the k0s-ansible repository:

```ShellSession
$ git clone https://github.com/movd/k0s-ansible.git
$ cd k0s-ansible
```

## Create virtual machines

_For this tutorial, multipass was used. However, there is no interdependence. This playbook should also work with VMs created in alternative ways or Raspberry Pis._

Next, create a couple of virtual machines. For the automation to work, each instance must have passwordless SSH access. To achieve this, we provision each instance with a cloud-init manifest that imports your current users' public SSH key and into a user `k0s`. For your convenience, a bash script is included that does just that:

`./tools/multipass_create_instances.sh 7` ◀️ this creates 7 virtual machines

```ShellSession
$ ./tools/multipass_create_instances.sh 7
Create cloud-init to import ssh key...
[1/7] Creating instance k0s-1 with multipass...
Launched: k0s-1
[2/7] Creating instance k0s-2 with multipass...
Launched: k0s-2
[3/7] Creating instance k0s-3 with multipass...
Launched: k0s-3
[4/7] Creating instance k0s-4 with multipass...
Launched: k0s-4
[5/7] Creating instance k0s-5 with multipass...
Launched: k0s-5
[6/7] Creating instance k0s-6 with multipass...
Launched: k0s-6
[7/7] Creating instance k0s-7 with multipass...
Launched: k0s-7
Name State IPv4 Image
k0s-1 Running 192.168.64.32 Ubuntu 20.04 LTS
k0s-2 Running 192.168.64.33 Ubuntu 20.04 LTS
k0s-3 Running 192.168.64.56 Ubuntu 20.04 LTS
k0s-4 Running 192.168.64.57 Ubuntu 20.04 LTS
k0s-5 Running 192.168.64.58 Ubuntu 20.04 LTS
k0s-6 Running 192.168.64.60 Ubuntu 20.04 LTS
k0s-7 Running 192.168.64.61 Ubuntu 20.04 LTS
```

## Create Ansible inventory

After that, we create our inventory directory by copying the sample:

```ShellSession
$ cp -rfp inventory/sample inventory/multipass
```

Now we need to create our inventory. The before built virtual machines need to be assigned to the different host groups required by the playbook's logic.

- `initial_controller` = must contain a single node that creates the worker and server tokens needed by the other nodes.
- `controller` = can contain nodes that, together with the host from `initial_controller` form a highly available isolated control plane.
- `worker` = must contain at least one node so that we can deploy Kubernetes objects.

We could fill `inventory/multipass/inventory.yml` by hand with the metadata provided by `multipass list,` but since we are lazy and want to automate as much as possible, we can use the included Python script `multipass_generate_inventory.py`:

To automatically fill our inventory run:

```
$ ./tools/multipass_generate_inventory.py
Designate first three instances as control plane
Created Ansible Inventory at: /Users/dev/k0s-ansible/tools/inventory.yml
$ cp tools/inventory.yml inventory/multipass/inventory.yml
```

Now `inventory/multipass/inventory.yml` should look like this (Of course, your IP addresses might differ):

```yaml
---
all:
  children:
    initial_controller:
      hosts:
        k0s-1:
    controller:
      hosts:
        k0s-2:
        k0s-3:
    worker:
      hosts:
        k0s-4:
        k0s-5:
        k0s-6:
        k0s-7:
  hosts:
    k0s-1:
      ansible_host: 192.168.64.32
    k0s-2:
      ansible_host: 192.168.64.33
    k0s-3:
      ansible_host: 192.168.64.56
    k0s-4:
      ansible_host: 192.168.64.57
    k0s-5:
      ansible_host: 192.168.64.58
    k0s-6:
      ansible_host: 192.168.64.60
    k0s-7:
      ansible_host: 192.168.64.61
  vars:
    ansible_user: k0s
```

## Test the connection to the virtual machines

To test the connection to your hosts just run:

```ShellSession
$ ansible -i inventory/multipass/inventory.yml -m ping
k0s-4 | SUCCESS => {
    "ansible_facts": {
        "discovered_interpreter_python": "/usr/bin/python3"
    },
    "changed": false,
    "ping": "pong"
}
...
```

If all is green and successful, you can proceed.

## Provision the cluster with Ansible

Finally, we can start provisioning the cluster. Applying the playbook, k0s will get downloaded and set up on all nodes, tokens will get exchanged, and a kubeconfig will get dumped to your local deployment environment.

```ShellSession
$ ansible-playbook site.yml -i inventory/multipass/inventory.yml
...
TASK [k0s/initial_controller : print kubeconfig command] *******************************************************
Tuesday 22 December 2020  17:43:20 +0100 (0:00:00.257)       0:00:41.287 ******
ok: [k0s-1] => {
    "msg": "To use Cluster: export KUBECONFIG=/Users/dev/k0s-ansible/inventory/multipass/artifacts/k0s-kubeconfig.yml"
}
...
PLAY RECAP *****************************************************************************************************
k0s-1                      : ok=21   changed=11   unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
k0s-2                      : ok=10   changed=5    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
k0s-3                      : ok=10   changed=5    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
k0s-4                      : ok=9    changed=5    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
k0s-5                      : ok=9    changed=5    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
k0s-6                      : ok=9    changed=5    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0
k0s-7                      : ok=9    changed=5    unreachable=0    failed=0    skipped=1    rescued=0    ignored=0

Tuesday 22 December 2020  17:43:36 +0100 (0:00:01.204)       0:00:57.478 ******
===============================================================================
prereq : Install apt packages -------------------------------------------------------------------------- 22.70s
k0s/controller : Wait for k8s apiserver ----------------------------------------------------------------- 4.30s
k0s/initial_controller : Create worker join token ------------------------------------------------------- 3.38s
k0s/initial_controller : Wait for k8s apiserver --------------------------------------------------------- 3.36s
download : Download k0s binary k0s-v0.9.0-rc1-amd64 ----------------------------------------------------- 3.11s
Gathering Facts ----------------------------------------------------------------------------------------- 2.85s
Gathering Facts ----------------------------------------------------------------------------------------- 1.95s
prereq : Create k0s Directories ------------------------------------------------------------------------- 1.53s
k0s/worker : Enable and check k0s service --------------------------------------------------------------- 1.20s
prereq : Write the k0s config file ---------------------------------------------------------------------- 1.09s
k0s/initial_controller : Enable and check k0s service --------------------------------------------------- 0.94s
k0s/controller : Enable and check k0s service ----------------------------------------------------------- 0.73s
Gathering Facts ----------------------------------------------------------------------------------------- 0.71s
Gathering Facts ----------------------------------------------------------------------------------------- 0.66s
Gathering Facts ----------------------------------------------------------------------------------------- 0.64s
k0s/worker : Write the k0s token file on worker --------------------------------------------------------- 0.64s
k0s/worker : Copy k0s service file ---------------------------------------------------------------------- 0.53s
k0s/controller : Write the k0s token file on controller ------------------------------------------------- 0.41s
k0s/controller : Copy k0s service file ------------------------------------------------------------------ 0.40s
k0s/initial_controller : Copy k0s service file ---------------------------------------------------------- 0.36s
```

## Use the cluster with kubectl

While the playbook ran, a kubeconfig got copied to your local machine. You can use it to get simple access to your new Kubernetes cluster:

```ShellSession
$ export KUBECONFIG=/Users/dev/k0s-ansible/inventory/multipass/artifacts/k0s-kubeconfig.yml
$ kubectl cluster-info
Kubernetes control plane is running at https://192.168.64.32:6443
CoreDNS is running at https://192.168.64.32:6443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
Metrics-server is running at https://192.168.64.32:6443/api/v1/namespaces/kube-system/services/https:metrics-server:/proxy

$ kubectl get nodes -o wide
NAME    STATUS     ROLES    AGE   VERSION        INTERNAL-IP     EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION     CONTAINER-RUNTIME
k0s-4   Ready      <none>   21s   v1.20.1-k0s1   192.168.64.57   <none>        Ubuntu 20.04.1 LTS   5.4.0-54-generic   containerd://1.4.3
k0s-5   Ready      <none>   21s   v1.20.1-k0s1   192.168.64.58   <none>        Ubuntu 20.04.1 LTS   5.4.0-54-generic   containerd://1.4.3
k0s-6   NotReady   <none>   21s   v1.20.1-k0s1   192.168.64.60   <none>        Ubuntu 20.04.1 LTS   5.4.0-54-generic   containerd://1.4.3
k0s-7   NotReady   <none>   21s   v1.20.1-k0s1   192.168.64.61   <none>        Ubuntu 20.04.1 LTS   5.4.0-54-generic   containerd://1.4.3
```

⬆️ Of course, the first three control plane nodes won't show up here because the control plane is fully isolated. You can check on the distributed etcd cluster by running this ad-hoc command (or ssh'ing directly into a controller node):

```ShellSession
$ ansible k0s-1 -a "k0s etcd member-list -c /etc/k0s/k0s.yaml" -i inventory/multipass/inventory.yml | tail -1 | jq
{
  "level": "info",
  "members": {
    "k0s-1": "https://192.168.64.32:2380",
    "k0s-2": "https://192.168.64.33:2380",
    "k0s-3": "https://192.168.64.56:2380"
  },
  "msg": "done",
  "time": "2020-12-23T00:21:22+01:00"
}
```

After a while, all worker nodes become `Ready`. Your cluster is now waiting to get used. We can test by creating a simple nginx deployment.

```ShellSession
$ kubectl create deployment nginx --image=gcr.io/google-containers/nginx --replicas=5
deployment.apps/nginx created

$ kubectl expose deployment nginx --target-port=80 --port=8100
service/nginx exposed

$ kubectl run hello-k0s --image=quay.io/prometheus/busybox --rm -it --restart=Never --command -- wget -qO- nginx:8100
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx on Debian!</title>
...
pod "hello-k0s" deleted
```

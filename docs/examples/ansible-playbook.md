# Creating a cluster with an Ansible Playbook

Ansible is a popular infrastructure-as-code tool that can use to automate tasks for the purpose of achieving the desired state in a system. With Ansible (and the k0s-Ansible playbook) you can quickly install a multi-node Kubernetes Cluster.

**Note**: Before using Ansible to create a cluster, you should have a general understanding of Ansible (refer to the official [Ansible User Guide](https://docs.ansible.com/ansible/latest/user_guide/index.html).

## Prerequisites

You will require the following tools to install k0s on local virtual machines:

| Tool            | Detail                                    |
|:----------------------|:------------------------------------------|
| `multipass`  | A lightweight VM manager that uses KVM on Linux, Hyper-V on Windows, and hypervisor.framework on macOS. [Installation information](https://multipass.run/docs)|
| `ansible`          | An infrastructure as code tool. [Installation Guide](https://docs.ansible.com/ansible/latest/installation_guide/index.html) |
| `kubectl`             | Command line tool for running commands against Kubernetes clusters.  [Kubernetes Install Tools](https://docs.ansible.com/ansible/latest/installation_guide/index.html) |

## Create the cluster

1. Download k0s-ansible

    Clone the k0s-ansible repository on your local machine:

    ```shell
    git clone https://github.com/movd/k0s-ansible.git
    cd k0s-ansible
    ```

2. Create virtual machines

    **Note**: Though multipass is the VM manager in use here, there is no interdependence.

    Create a number of virtual machines. For the automation to work, each instance must have passwordless SSH access. To achieve this, provision each instance with a cloud-init manifest that imports your current users' public SSH key and into a user `k0s` (refer to the bash script below).

    This creates 7 virtual machines:

    ```shell
    ./tools/multipass_create_instances.sh 7
    ```

    ```shell
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

3. Create Ansible inventory

    1. Copy the sample to create the inventory directory:

        ```shell
        cp -rfp inventory/sample inventory/multipass
        ```

    2. Create the inventory.

        Assign the virtual machines to the different host groups, as required by the playbook logic.

        | Host group            | Detail                                    |
        |:----------------------|:------------------------------------------|
        | `initial_controller`  | Must contain a single node that creates the worker and controller tokens needed by the other nodes|
        | `controller`          | Can contain nodes that, together with the host from `initial_controller`, form a highly available isolated control plane |
        | `worker`              | Must contain at least one node, to allow for the deployment of Kubernetes objects |

    3. Fill in `inventory/multipass/inventory.yml`. This can be done by direct entry using the metadata provided by `multipass list,`, or you can use the following Python script `multipass_generate_inventory.py`:

        ```shell
        ./tools/multipass_generate_inventory.py
        ```

        ```shell
        Designate first three instances as control plane
        Created Ansible Inventory at: /Users/dev/k0s-ansible/tools/inventory.yml
        $ cp tools/inventory.yml inventory/multipass/inventory.yml
        ```

        Your `inventory/multipass/inventory.yml` should resemble the example below:

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

4. Test the virtual machine connections

    Run the following command to test the connection to your hosts:

    ```shell
    ansible -i inventory/multipass/inventory.yml -m ping
    ```

    ```shell
    k0s-4 | SUCCESS => {
        "ansible_facts": {
            "discovered_interpreter_python": "/usr/bin/python3"
        },
        "changed": false,
        "ping": "pong"
    }
    ...
    ```

    If the test result indicates success, you can proceed.

5. Provision the cluster with Ansible

    Applying the playbook, k0s download and be set up on all nodes, tokens will be exchanged, and a kubeconfig will be dumped to your local deployment environment.

    ```shell
    ansible-playbook site.yml -i inventory/multipass/inventory.yml
    ```

    ```shell
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

A kubeconfig was copied to your local machine while the playbook was running which you can use to gain access to your new Kubernetes cluster:

```shell
export KUBECONFIG=/Users/dev/k0s-ansible/inventory/multipass/artifacts/k0s-kubeconfig.yml
kubectl cluster-info
```

```shell
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

**Note**: The first three control plane nodes will not display, as the control plane is fully isolated. To check on the distributed etcd cluster, you can use ssh to securely log a controller node, or you can run the following ad-hoc command:

```shell
ansible k0s-1 -a "k0s etcd member-list -c /etc/k0s/k0s.yaml" -i inventory/multipass/inventory.yml | tail -1 | jq
```

```json
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

Once all worker nodes are at `Ready` state you can use the cluster. You can test the cluster state by creating a simple nginx deployment.

```shell
kubectl create deployment nginx --image=gcr.io/google-containers/nginx --replicas=5
```

```shell
deployment.apps/nginx created
```

```shell
kubectl expose deployment nginx --target-port=80 --port=8100
```

```shell
service/nginx exposed
```

```shell
kubectl run hello-k0s --image=quay.io/prometheus/busybox --rm -it --restart=Never --command -- wget -qO- nginx:8100
```

```shell
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx on Debian!</title>
...
pod "hello-k0s" deleted
```

**Note**: k0s users are the developers of k0s-ansible. Please send your feedback, bug reports, and pull requests to [github.com/movd/k0s-ansible](https://github.com/movd/k0s-ansible)._

# Using Terraform to manage deployment of your k0s cluster

[Terraform](https://www.terraform.io/) is a popular tool to deploy resource using resources that are pre-made and easily adabtable to your enviroment. Some are complex, and some are simple. We aim to provide here a simple Terraform script that can deploy k0s using k0sctl.

## Assumptions and Requirements

First this document will not help you get your machine ready for using terraform or any other tools listed below, but it assumes you have knnownedge to get them working and also the infrastructure ready to deploy k0s. This script does not automate creating new servers/hosts on any cloud provider or local virtualized enviroment.

To use the script below make sure you have:
- [Terraform](https://www.terraform.io/) installed
- [k0sctl](k0sctl-install.md) installed already
- generated ssh keys deployed to your "hosts"
- proper network and stable connection to your enviroment
- DNS understanding if you plan to use FQDN in the Terraform script

If you plan on running more than one controller and/or controller/worker, be sure to have a load balancer in place. You can read about setting up a load balancer [here](high-availability.md#load-balancer) for your options. The load balancer must be setup and working prior to setting up a multi controller k0s enviroment.

## Understanding Terraform File

The terraform "start" file below can accept DNS entries of your server (as long as they are resolvable among all nodes) or the IP address of the nodes. It's broken down into parts.

### Vars

These hold static non-changing information that can change based off your custom requirements,b ut for the most part, even if you were going to "redeploy" the terraform script, 99% of the time they will not be changing.

### Load Balancer

This part assumes you are using DNS entrie which will be resolvable by DNS or other method and resolve to the IP address of the load balancer VIP itself where the controllers will communicate.

```txt
# Load Balancer

data "dns_a_record_set" "lb_address" {
  host = var.lb_address
}
```

Please note that this section can be removed if you are using an IP address for the load balancer (lb_address) variable.

### Controllers and Workers

```txt
# Controllers

data "dns_a_record_set" "controller01" {
  host = "[FQDN DNS NAME]"
}

# Workers

data "dns_a_record_set" "worker01" {
  host = "[FQDN DNS NAME]"
}
```

In this section you need to spefficify the DNS enerties of each of your servers. Again, be resolvable by DNS or other method in order to use this method of using DNS. Otherwise you can remove it.

### Main Part

This is the bare miniam strcture that you need in order to use terraform. Obvisally from this snippiet of the full example at the [bottom of this page](#Full Example) is missing the bloxks needed for what needs to go inside the ```hosts``` block.

```txt
locals {
  k0s_tmpl = {
      apiVersion = "k0sctl.k0sproject.io/v1beta1"
      kind = "cluster"
      spec = {
          hosts = [
            ...
           ]
      }
  }
}

output "k0s_cluster" {
  value = yamlencode(local.k0s_tmpl)
}
```
Save this file called k0s-terraform.tf and note the directory your saving it in. ```root``` user needs access to this file. Keep in mind that this is abasic strcture of k0s only and your terraform file might be more complex and can do other things, but the entire file must be properlly formated and understood by terraform if the structure is off.

#### Hosts

Each ```host``` block needs to follow this strcture:

```txt
{
  ssh = {
    address  = "address of the host"
    user     = "root"
    keyPath = var.ssh_key # this comes from the variable
  }
  role = "ROLETYPE"
},
```
ROLETYPE can be controler+worker, controller, or worker.

If you are to use the DNS entries to resolve the FQDN to IP it would look like this:

- ```join(",", data.dns_a_record_set.controller01.addrs)```

Where ```controller01``` is the variable name for the DNS Entiries. This would go in place of the IP address in the address field. Once you added at least one controler or controller/worker and/or a worker to this block your nromal deployment process of k0s using k0sctl will work correctly if everything else is inline.

### Custom Config

If you need to provide a custom config into your k0s or are setting k0s with more than one controller, your config must have the LB information placed into certain fields. Those would be externalAddress and the "sans" fields for certificate acceptace by the controllers themselfs.

After the spec section add in:

```txt
          k0s = {
              "config" = {
                  "apiVersion" = "k0s.k0sproject.io/v1beta1"
                  "kind" =  "Cluster"
                  "metadata" = {
                      "name" = var.cluster_name
                  }
                  "spec" = {
                      "api" = {
                          "externalAddress" = "address of the LB"
                          "sans" = "address of the LB"
                      }
                  }
              }
          }
```

This is the bare minimum needed for load balancer, but any addtionial options that you may want to apply can be applied as long as it's vaild terraform format and a vaild k0s [config option](configuration.md). 

### Running Terraform to pocess your file

Now your ready to run Terraform. Make sure your running as ```root``` or using sudo. In the directory where your terraform file is (eg. k0s-terraform.tf) enter:

- ```terraform init```

This will initailze terraform to download any requirement modules that are in the file. Once that is complete your ready to process the terraform file to install k0s.

You can test our your terraform file is good by getting a sample output of what the YAML generated needed for k0sctl will look like:

- ```terraform output -raw k0s_cluster```

Should output something simular to this if everything is correct:

```txt
root@k0sctrl:~# terraform output -raw k0s_cluster
"apiVersion": "k0sctl.k0sproject.io/v1beta1"
"kind": "cluster"
"spec":
  "hosts":
  - "role": "controller"
    "ssh":
      "address": "10.0.0.1"
      "keyPath": "~/.ssh/id_rsa"
      "user": "root"
  - "role": "worker"
    "ssh":
      "address": "10.0.0.2"
      "keyPath": "~/.ssh/id_rsa"
      "user": "root"
```
As you can see everything is properlly formated in the correct method. You can pipe out the resolve into a file and this with you k0sctl execuable run the output file with k0sctl for it to process. Another method is to pipe it into k0sctl directly:

-  ```terraform output -raw k0s_cluster | ./k0sctl apply --config -```

Which will run ```k0sctl``` to start building your k0s structure based off the output generated.

If everything is is done correctly, you should see the output of k0sctrl installing k0s among your hosts and output that your k0s install was successful.

### Getting your Kube Config

Now that your k0s setup is deployed, you need your kube config file in order to connect to the enviroment. You may do this by running this command:

- ```terraform output -raw k0s_cluster | ./k0sctl kubeconfig --config -```

Which will export to your screen the config. If you want to pipe it into a file after the dash (-) at the end of the command above add in ```> kube-config.conf``` and make sure that this file is loaded into your kubectl enviroment variable or piped in each and everytime in order to run commandss. You can even download this file or output into Lens to access your enviroment.

## Updating CLuster

As long as your format doesn't change you can add in hosts at any time and re-runt he commands to process the terraform file, and k0sctl will go through your entire enviroment and add those addtionial nodes without having to bring down your enviroment.

Run commands in this order:

` terraform init
- terraform plan
- terrafor apply

Once you run those commands you can reapply to the k0s cluster using k0sctl:

-  ```terraform output -raw k0s_cluster | ./k0sctl apply --config -```

This also is true for keeping k0s updated as it is based of k0sctrl "k0s" version that will be deployed as long as you are not overrideing that in your terraform config file. You wouldn't need to be update the terraform config files as k0sctl will re-run once it's updated with the latest k0s version on your current nodes and config.

## Removing Nodes

At this moment, k0sctl can not remove nodes from the cluster. If you are removing the nodes, be sure to manually follow manual steps to remove the node form the k0s cluster. Be sure to re-run terraform without the node that your removed so that any future running of the terraform script does not try to create and/or reconfigure your k0s setup with invaild non-working information. 

## Full Example

```txt
# Vars

variable "cluster_name" {
  default = "k0s_prod"
}

variable "ssh_key" {
  default = "~/.ssh/id_rsa"
}

variable "lb_address" {
  default = "[FQDN DNS NAME or IP ADDRESS]"
}

# Load Balancer

data "dns_a_record_set" "lb_address" {
  host = var.lb_address
}

# Controllers

data "dns_a_record_set" "controller01" {
  host = "[FQDN DNS NAME]"
}

# Workers

data "dns_a_record_set" "worker01" {
  host = "[FQDN DNS NAME]"
}

locals {
  k0s_tmpl = {
      apiVersion = "k0sctl.k0sproject.io/v1beta1"
      kind = "cluster"
      spec = {
          hosts = [
              {
                ssh = {
                  address  = join(",", data.dns_a_record_set.controller01.addrs)
                  user     = "root"
                  keyPath = var.ssh_key
                }
                role = "controller"
              }, {
                ssh = {
                  address  = join(",", data.dns_a_record_set.worker01.addrs)
                  user     = "root"
                  keyPath = var.ssh_key
                }
                role = "worker"
              }
          ]
          k0s = {
              version = "0.12.1"
              "config" = {
                  "apiVersion" = "k0s.k0sproject.io/v1beta1"
                  "kind" =  "Cluster"
                  "metadata" = {
                      "name" = var.cluster_name
                  }
                  "spec" = {
                      "api" = {
                          "externalAddress" = join(",", data.dns_a_record_set.lb_address.addrs)
                          "sans" = [join(",", data.dns_a_record_set.lb_address.addrs), var.lb_address]
                      }
                  }
              }
          }
      }
  }
}

output "k0s_cluster" {
  value = yamlencode(local.k0s_tmpl)
}
```

provider "aws" {
  default_tags {
    tags = merge(var.additional_tags, {
      "ostests.k0sproject.io/instance"             = local.resource_name_prefix
      "ostests.k0sproject.io/os"                   = var.os
      "ostests.k0sproject.io/k0s-network-provider" = var.k0s_network_provider
      "ostests.k0sproject.io/k0s-kube-proxy-mode"  = var.k0s_kube_proxy_mode
    })
  }
}

resource "random_pet" "resource_name_prefix" {
  count = var.resource_name_prefix == null ? 1 : 0
}

locals {
  resource_name_prefix = coalesce(var.resource_name_prefix, random_pet.resource_name_prefix.*.id...)
  cache_dir            = pathexpand(coalesce(var.cache_dir, "~/.cache/k0s-ostests"))
  podCIDR              = "10.244.0.0/16"


  hosts                    = try(module.infra.nodes, []) # the try allows destruction even if infra provisioning failed
  ssh_private_key_filename = local_sensitive_file.ssh_private_key.filename
}

module "os" {
  source = "./modules/os"

  os                       = var.os
  arch                     = var.arch
  additional_ingress_cidrs = [local.podCIDR]
}

module "infra" {
  source = "./modules/infra"

  resource_name_prefix     = local.resource_name_prefix
  os                       = module.os.os
  additional_ingress_cidrs = [local.podCIDR]
}

resource "local_sensitive_file" "ssh_private_key" {
  content         = module.infra.ssh_private_key
  filename        = "${local.cache_dir}/aws-${local.resource_name_prefix}-ssh-private-key.pem"
  file_permission = "0400"
}

module "k0sctl" {
  count = var.k0sctl_skip ? 0 : 1

  source = "./modules/k0sctl"

  k0sctl_executable_path = var.k0sctl_executable_path
  k0s_executable_path    = var.k0s_executable_path
  k0s_version            = var.k0s_version

  k0s_config_spec = {
    network = {
      provider = var.k0s_network_provider
      podCIDR  = local.podCIDR

      kubeProxy = {
        mode = var.k0s_kube_proxy_mode
      }

      nodeLocalLoadBalancing = {
        enabled = true
      }
    }
  }

  hosts                    = local.hosts
  ssh_private_key_filename = local.ssh_private_key_filename
}

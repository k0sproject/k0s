locals {
  k0s_spec_images_airgap = var.k0s_airgap_bundle_config != "" ? yamldecode(file(format("/tool/data/%s", var.k0s_airgap_bundle_config))) : null

  k0s_tmpl = {
    apiVersion = "k0sctl.k0sproject.io/v1beta1"
    kind       = "cluster"
    spec = {
      hosts = [
        for host in concat(module.k0sinfra.controllers, module.k0sinfra.workers) : {
          ssh = {
            address = host.public_ip
            user    = "ubuntu"
            keyPath = "/tool/data/private.pem"
          }
          role          = host.tags["Role"]
          uploadBinary  = var.k0s_binary != "" ? true : false
          k0sBinaryPath = format("/tool/data/%s", var.k0s_binary)
          files = [
            for val in (var.k0s_airgap_bundle != "" ? ["airgap"] : []) : {
              name = "image-bundle"
              src = format("/tool/data/%s", var.k0s_airgap_bundle)
              dstDir = "/var/lib/k0s/images/"
              perm = "0755"
            }
          ]
        }
      ]
      k0s = {
        version = var.k0s_version
        config = {
          apiVersion = "k0s.k0sproject.io/v1beta1"
          kind       = "Cluster"
          metadata = {
            name = "k0s"
          }
          spec = {
            api = {
              externalAddress = module.k0sinfra.loadbalancer_dns
            }
            images = var.k0s_airgap_bundle != "" ? local.k0s_spec_images_airgap : null
          }
        }
      }
    }
  }
}

output "k0s_cluster" {
  value = yamlencode(local.k0s_tmpl)
}

output "k0s_update_binary_url" {
  value = length(module.s3updateserver) > 0 ? module.s3updateserver[0].k0s_update_binary_url : ""
}

output "k0s_update_airgap_bundle_url" {
  value = length(module.s3updateserver) > 0 ? module.s3updateserver[0].k0s_update_airgap_bundle_url : ""
}

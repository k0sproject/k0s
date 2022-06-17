
data "http" "k0s_version" {
  count = local.use_remote_k0s_version ? 1 : 0
  url   = "https://docs.k0sproject.io/${var.k0s_version}.txt"
}

locals {
  use_remote_k0s_version = var.k0s_version == "stable" || var.k0s_version == "latest"
  k0sctl_tmpl = {
    apiVersion = "k0sctl.k0sproject.io/v1beta1"
    kind       = "cluster"
    metadata = {
      name = local.cluster_unique_identifier
    }
    spec = {
      hosts = [
        for host in concat(aws_eip.controller-ext, aws_instance.cluster-workers) : {
          ssh = {
            address = host.public_ip
            user    = "ubuntu"
            keyPath = "./aws_private.pem"
          }
          role          = host.tags["Role"]
          uploadBinary  = true
          k0sBinaryPath = local.use_remote_k0s_version ? null : "../../../k0s"
        }
      ]
      k0s = {
        version = local.use_remote_k0s_version ? chomp(data.http.k0s_version.0.body) : var.k0s_version
      }
    }
  }
}

// Save the private key to filesystem
resource "local_file" "k0sctl_config" {
  file_permission = "600"
  filename        = format("%s/%s", path.module, "k0sctl.yaml")
  content         = yamlencode(local.k0sctl_tmpl)
}

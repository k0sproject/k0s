locals {
  use_remote_k0s_version = var.k0s_version == "stable" || var.k0s_version == "latest"
}

data "http" "k0s_version" {
  count = local.use_remote_k0s_version ? 1 : 0
  url   = "https://docs.k0sproject.io/${var.k0s_version}.txt"
}

locals {
  k0s_version = local.use_remote_k0s_version ? chomp(one(data.http.k0s_version).response_body) : var.k0s_version

  k0sctl_config = {
    apiVersion = "k0sctl.k0sproject.io/v1beta1"
    kind       = "Cluster"
    metadata   = { name = "k0s-cluster" }
    spec = {
      k0s = {
        version = local.k0s_version
        config = { spec = merge(
          { telemetry = { enabled = false, }, },
          { for k, v in var.k0s_config_spec : k => v if v != null }
        ), }
      }

      hosts = [for host in var.hosts : merge(
        {
          role         = host.role
          uploadBinary = var.k0s_executable_path != null
        },

        host.connection.type != "ssh" ? {} : merge({
          ssh = {
            address = host.ipv4
            keyPath = var.ssh_private_key_filename
            port    = 22
            user    = host.connection.username
          }
        }),

        var.k0s_executable_path == null ? {} : {
          k0sBinaryPath = var.k0s_executable_path
        },
      )]
    }
  }
}

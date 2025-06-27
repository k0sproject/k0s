# https://docs.fedoraproject.org/en-US/fedora-coreos/provisioning-aws/

data "http" "fcos_stable_stream" {
  count = var.os == "fcos_stable" ? 1 : 0
  url   = "https://builds.coreos.fedoraproject.org/streams/stable.json"
}

data "aws_region" "fcos_stable" {}

locals {
  os_fcos_stable = var.os != "fcos_stable" ? {} : {
    node_configs = {
      default = {
        ami_id = jsondecode(one(data.http.fcos_stable_stream).body).architectures[var.arch == "arm64" ? "aarch64" : var.arch].images.aws.regions[data.aws_region.fcos_stable.name].image

        connection = {
          type     = "ssh"
          username = "core"
        }
      }
    }
  }
}

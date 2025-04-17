# https://docs.fedoraproject.org/en-US/fedora-coreos/provisioning-aws/

data "aws_ami" "fcos_41" {
  count = var.os == "fcos_41" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^fedora-coreos-41\\.\\d+\\..+-x86_64"
  most_recent = true

  filter {
    name   = "name"
    values = ["fedora-coreos-41.*.*-x86_64"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  lifecycle {
    precondition {
      condition     = var.arch == "x86_64"
      error_message = "Unsupported architecture for Fedora CoreOS 41."
    }
  }
}

locals {
  os_fcos_41 = var.os != "fcos_41" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.fcos_41.*.id)

        connection = {
          type     = "ssh"
          username = "core"
        }
      }
    }
  }
}

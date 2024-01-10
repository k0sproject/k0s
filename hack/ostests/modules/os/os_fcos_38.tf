# https://docs.fedoraproject.org/en-US/fedora-coreos/provisioning-aws/

data "aws_ami" "fcos_38" {
  count = var.os == "fcos_38" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^fedora-coreos-38\\.\\d+\\..+-x86_64"
  most_recent = true

  filter {
    name   = "name"
    values = ["fedora-coreos-38.*.*-x86_64"]
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
      error_message = "Unsupported architecture for Fedora CoreOS 38."
    }
  }
}

locals {
  os_fcos_38 = var.os != "fcos_38" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.fcos_38.*.id)

        connection = {
          type     = "ssh"
          username = "core"
        }
      }
    }
  }
}

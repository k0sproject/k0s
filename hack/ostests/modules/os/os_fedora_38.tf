# https://alt.fedoraproject.org/cloud/

data "aws_ami" "fedora_38" {
  count = var.os == "fedora_38" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^Fedora-Cloud-Base-38-.+\\.x86_64-hvm-"
  most_recent = true

  filter {
    name   = "name"
    values = ["Fedora-Cloud-Base-38-*.x86_64-hvm-*"]
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
      error_message = "Unsupported architecture for Fedora Linux 38 (Cloud Edition)."
    }
  }
}

locals {
  os_fedora_38 = var.os != "fedora_38" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.fedora_38.*.id)

        connection = {
          type     = "ssh"
          username = "fedora"
        }
      }
    }
  }
}

# https://alt.fedoraproject.org/cloud/

data "aws_ami" "fedora_41" {
  count = var.os == "fedora_41" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^Fedora-Cloud-Base-AmazonEC2.x86_64-41-.+"
  most_recent = true

  filter {
    name   = "name"
    values = ["Fedora-Cloud-Base-AmazonEC2.x86_64-41-*"]
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
      error_message = "Unsupported architecture for Fedora Linux 41 (Cloud Edition)."
    }
  }
}

locals {
  os_fedora_41 = var.os != "fedora_41" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.fedora_41.*.id)

        connection = {
          type     = "ssh"
          username = "fedora"
        }
      }
    }
  }
}

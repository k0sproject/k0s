# https://alt.fedoraproject.org/cloud/

data "aws_ami" "fedora_45" {
  count = var.os == "fedora_45" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^Fedora-Cloud-Base-AmazonEC2.x86_64-45-.+"
  most_recent = true

  filter {
    name   = "name"
    values = ["Fedora-Cloud-Base-AmazonEC2.x86_64-45-*"]
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
      error_message = "Unsupported architecture for Fedora Linux 45 (Cloud Edition)."
    }
  }
}

locals {
  os_fedora_45 = var.os != "fedora_45" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.fedora_45.*.id)

        connection = {
          type     = "ssh"
          username = "fedora"
        }
      }
    }
  }
}

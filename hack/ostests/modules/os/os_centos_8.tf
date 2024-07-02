# https://centos.org/download/aws-images/

data "aws_ami" "centos_8" {
  count = var.os == "centos_8" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^CentOS Stream 8 x86_64 \\d+"
  most_recent = true

  filter {
    name   = "name"
    values = ["CentOS Stream 8 x86_64 *"]
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
      error_message = "Unsupported architecture for CentOS Stream 8."
    }
  }
}

locals {
  os_centos_8 = var.os != "centos_8" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.centos_8.*.id)

        connection = {
          type     = "ssh"
          username = "centos"
        }
      }
    }
  }
}

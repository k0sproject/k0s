# https://centos.org/download/aws-images/

data "aws_ami" "centos_7" {
  count = var.os == "centos_7" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^CentOS Linux 7 x86_64 - "
  most_recent = true

  filter {
    name   = "name"
    values = ["CentOS Linux 7 x86_64 - *"]
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
      error_message = "Unsupported architecture for CentOS Linux 7 (Core)."
    }
  }
}

locals {
  os_centos_7 = var.os != "centos_7" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.centos_7.*.id)

        connection = {
          type     = "ssh"
          username = "centos"
        }
      }
    }
  }
}

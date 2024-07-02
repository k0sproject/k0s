# https://www.flatcar.org/docs/latest/installing/cloud/aws-ec2/

data "aws_ami" "flatcar" {
  count = var.os == "flatcar" ? 1 : 0

  owners      = ["075585003325"]
  name_regex  = "^Flatcar-stable-\\d+\\..+-hvm"
  most_recent = true

  filter {
    name   = "name"
    values = ["Flatcar-stable-*.*-hvm"]
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
      error_message = "Unsupported architecture for Flatcar Container Linux."
    }
  }
}

locals {
  os_flatcar = var.os != "flatcar" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.flatcar.*.id)

        connection = {
          type     = "ssh"
          username = "core"
        }
      }
    }
  }
}

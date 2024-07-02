# https://rockylinux.org/cloud-images/

data "aws_ami" "rocky_8" {
  count = var.os == "rocky_8" ? 1 : 0

  owners      = ["792107900819"]
  name_regex  = "^Rocky-8-EC2-Base-8\\.7-\\d+\\.\\d+\\.x86_64"
  most_recent = true

  filter {
    name   = "name"
    values = ["Rocky-8-EC2-Base-8.7-*.x86_64"]
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
      error_message = "Unsupported architecture for Rocky Linux 8.7 (Green Obsidian)."
    }
  }
}

locals {
  os_rocky_8 = var.os != "rocky_8" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.rocky_8.*.id)

        connection = {
          type     = "ssh"
          username = "rocky"
        }
      }
    }
  }
}

# https://wiki.debian.org/Cloud/AmazonEC2Image/Bookworm

data "aws_ami" "debian_12" {
  count = var.os == "debian_12" ? 1 : 0

  owners      = ["136693071363"]
  name_regex  = "^debian-12-(amd64|arm64)-\\d+-\\d+$"
  most_recent = true

  filter {
    name   = "name"
    values = ["debian-12-*-*-*"]
  }

  filter {
    name   = "architecture"
    values = [var.arch]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

locals {
  os_debian_12 = var.os != "debian_12" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.debian_12.*.id)

        user_data = format("#cloud-config\n%s", jsonencode({
          runcmd = ["truncate -s0 /etc/motd", ]
        })),

        connection = {
          type     = "ssh"
          username = "admin"
        }
      }
    }
  }
}

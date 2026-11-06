# https://wiki.debian.org/Cloud/AmazonEC2Image/Trixie

data "aws_ami" "debian_13" {
  count = var.os == "debian_13" ? 1 : 0

  owners      = ["136693071363"]
  name_regex  = "^debian-13-(amd64|arm64)-\\d+-\\d+$"
  most_recent = true

  filter {
    name   = "name"
    values = ["debian-13-*-*-*"]
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
  os_debian_13 = var.os != "debian_13" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.debian_13.*.id)

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

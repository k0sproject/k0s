# https://wiki.debian.org/Cloud/AmazonEC2Image/Bullseye

data "aws_ami" "debian_11" {
  count = var.os == "debian_11" ? 1 : 0

  owners      = ["136693071363"]
  name_regex  = "^debian-11-amd64-\\d+-\\d+$"
  most_recent = true

  filter {
    name   = "name"
    values = ["debian-11-amd64-*-*"]
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
      error_message = "Unsupported architecture for Debian GNU/Linux 11 (bullseye)."
    }
  }
}

locals {
  os_debian_11 = var.os != "debian_11" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.debian_11.*.id)

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

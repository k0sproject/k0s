# https://ubuntu.com/server/docs/cloud-images/amazon-ec2

data "aws_ami" "ubuntu_2004" {
  count = var.os == "ubuntu_2004" ? 1 : 0

  owners      = ["099720109477"]
  name_regex  = "ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-\\d+"
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
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
      error_message = "Unsupported architecture for Ubuntu 20.04 LTS."
    }
  }
}

locals {
  os_ubuntu_2004 = var.os != "ubuntu_2004" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.ubuntu_2004.*.id)

        user_data = format("#cloud-config\n%s", jsonencode({
          runcmd = [
            "find /etc/update-motd.d -maxdepth 1 -executable \\( -type f -o -type l \\) -exec chmod -x '{}' \\;",
          ]
        })),

        connection = {
          type     = "ssh"
          username = "ubuntu"
        }
      }
    }
  }
}

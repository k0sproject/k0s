# https://rockylinux.org/cloud-images/

data "aws_ami" "rocky_10" {
  count = var.os == "rocky_10" ? 1 : 0

  owners      = ["792107900819"]
  name_regex  = "^Rocky-10-EC2-Base-10\\.2-\\d+\\.\\d+\\.x86_64"
  most_recent = true

  filter {
    name   = "name"
    values = ["Rocky-10-EC2-Base-10.2-*.x86_64"]
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
      error_message = "Unsupported architecture for Rocky Linux 10."
    }
  }
}

locals {
  os_rocky_10 = var.os != "rocky_10" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.rocky_10.*.id)

        connection = {
          type     = "ssh"
          username = "rocky"
        }
      }

      worker = {
        user_data = format("#cloud-config\n%s", jsonencode({
          # for nf_conntrack and friends
          runcmd = ["dnf install -y \"kernel-modules-extra-$(uname -r)\""]
        })),
      }
    }
  }
}

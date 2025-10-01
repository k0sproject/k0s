# https://access.redhat.com/solutions/15356

data "aws_ami" "rhel_9" {
  count = var.os == "rhel_9" ? 1 : 0

  owners      = ["309956199498"]
  name_regex  = "^RHEL-9\\.5\\.\\d+_HVM-\\d+-x86_64-"
  most_recent = true

  filter {
    name   = "name"
    values = ["RHEL-9.5.*_HVM-*-x86_64-*"]
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
      error_message = "Unsupported architecture for Red Hat Enterprise Linux 9.0 (Plow)."
    }
  }
}

locals {
  os_rhel_9 = var.os != "rhel_9" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.rhel_9.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
        }
      }
    }
  }
}

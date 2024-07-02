# https://access.redhat.com/solutions/15356

data "aws_ami" "rhel_7" {
  count = var.os == "rhel_7" ? 1 : 0

  owners      = ["309956199498"]
  name_regex  = "^RHEL-7\\.9_HVM-\\d+-x86_64-"
  most_recent = true

  filter {
    name   = "name"
    values = ["RHEL-7.9_HVM-*-x86_64-*"]
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
      error_message = "Unsupported architecture for Red Hat Enterprise Linux Server 7.9 (Maipo)."
    }
  }
}

locals {
  os_rhel_7 = var.os != "rhel_7" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.rhel_7.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
        }
      }
    }
  }
}

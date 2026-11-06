# https://access.redhat.com/solutions/15356

data "aws_ami" "rhel_10" {
  count = var.os == "rhel_10" ? 1 : 0

  owners      = ["309956199498"]
  name_regex  = "^RHEL-10\\.2\\.\\d+_HVM_GA-\\d+-x86_64-"
  most_recent = true

  filter {
    name   = "name"
    values = ["RHEL-10.2.*_HVM_GA-*-x86_64-*"]
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
      error_message = "Unsupported architecture for Red Hat Enterprise Linux 10."
    }
  }
}

locals {
  os_rhel_10 = var.os != "rhel_10" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.rhel_10.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
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

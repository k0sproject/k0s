# https://centos.org/download/aws-images/

data "aws_ami" "centos_10" {
  count = var.os == "centos_10" ? 1 : 0

  owners      = ["125523088429"]
  name_regex  = "^CentOS Stream 10 x86_64 \\d+"
  most_recent = true

  filter {
    name   = "name"
    values = ["CentOS Stream 10 x86_64 *"]
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
      error_message = "Unsupported architecture for CentOS Stream 10."
    }
  }
}

locals {
  os_centos_10 = var.os != "centos_10" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.centos_10.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
        }
      }

      worker = {
        user_data = format("#cloud-config\n%s", jsonencode({
          # for nf_conntrack and friends
          packages = ["kernel-modules-extra"]
        })),
      }
    }
  }
}

# https://docs.aws.amazon.com/linux/al2023/ug/naming-and-versioning.html

data "aws_ami" "al2023" {
  count = var.os == "al2023" ? 1 : 0

  owners      = ["137112412989"]
  name_regex  = "^al2023-ami-2023.\\d+.\\d+.\\d+-.*-x86_64"
  most_recent = true

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
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
      error_message = "Unsupported architecture for Amazon Linux 2023."
    }
  }
}

locals {
  os_al2023 = var.os != "al2023" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.al2023.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
        }
      }
    }
  }
}

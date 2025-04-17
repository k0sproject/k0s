data "aws_ami" "sles_15" {
  count = var.os == "sles_15" ? 1 : 0

  owners      = ["013907871322"]
  name_regex  = "^suse-sles-15-sp6-v\\d+-hvm-ssd-x86_64"
  most_recent = true

  filter {
    name   = "name"
    values = ["suse-sles-15-sp6-v*-hvm-ssd-x86_64"]
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
      error_message = "Unsupported architecture for SUSE Linux Enterprise Server 15 SP6."
    }
  }
}

locals {
  os_sles_15 = var.os != "sles_15" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.sles_15.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
        }
      }
    }
  }
}

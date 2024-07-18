# https://www.alpinelinux.org/cloud/

data "aws_ami" "alpine_3_20" {
  count = var.os == "alpine_3_20" ? 1 : 0

  owners      = ["538276064493"]
  name_regex  = "^alpine-3\\.20\\.\\d+-(aarch64|x86_64)-uefi-tiny($|-.*)"
  most_recent = true

  filter {
    name   = "name"
    values = ["alpine-3.20.*-*-uefi-tiny*"]
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
  os_alpine_3_20 = var.os != "alpine_3_20" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.alpine_3_20.*.id)

        user_data    = templatefile("${path.module}/os_alpine_userdata.tftpl", { worker = true })
        ready_script = file("${path.module}/os_alpine_ready.sh")

        connection = {
          type     = "ssh"
          username = "alpine"
        }
      }
      controller = {
        user_data = templatefile("${path.module}/os_alpine_userdata.tftpl", { worker = false })
      }
    }
  }
}

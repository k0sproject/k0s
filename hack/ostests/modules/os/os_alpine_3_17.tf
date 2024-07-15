# https://www.alpinelinux.org/cloud/

data "aws_ami" "alpine_3_17" {
  # Pin Alpine to 3.17.3 as something changed in 3.17.4 that prevents SSH logins.

  count = var.os == "alpine_3_17" ? 1 : 0

  owners      = ["538276064493"]
  name_regex  = "^alpine-3\\.17\\.3-x86_64-bios-tiny($|-.*)"
  most_recent = true

  filter {
    name   = "name"
    values = ["alpine-3.17.3-x86_64-bios-tiny*"]
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
      error_message = "Unsupported architecture for Alpine Linux 3.17."
    }
  }
}

locals {
  os_alpine_3_17 = var.os != "alpine_3_17" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.alpine_3_17.*.id)

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

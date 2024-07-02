# https://blogs.oracle.com/linux/post/running-oracle-linux-in-public-clouds
# https://forums.oracle.com/ords/apexds/post/launch-an-oracle-linux-instance-in-aws-9462

data "aws_ami" "oracle_7_9" {
  count = var.os == "oracle_7_9" ? 1 : 0

  owners      = ["131827586825"]
  name_regex  = "^OL7\\.9-x86_64-HVM-"
  most_recent = true

  filter {
    name   = "name"
    values = ["OL7.9-x86_64-HVM-*"]
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
      error_message = "Unsupported architecture for Oracle Linux Server 7.9."
    }
  }
}

locals {
  os_oracle_7_9 = var.os != "oracle_7_9" ? {} : {
    node_configs = {
      default = {
        ami_id = one(data.aws_ami.oracle_7_9.*.id)

        connection = {
          type     = "ssh"
          username = "ec2-user"
        }
      }

      controller = {
        user_data = format("#cloud-config\n%s", jsonencode({
          write_files = [{
            path    = "/etc/firewalld/services/k0s-controller.xml",
            content = file("${path.module}/k0s-controller.firewalld-service.xml"),
          }]

          runcmd = [
            "firewall-offline-cmd --add-service=k0s-controller",
            "systemctl reload firewalld.service",
          ]
        }))
      }

      "controller+worker" = {
        user_data = format("#cloud-config\n%s", jsonencode({
          write_files = [for role in ["controller", "worker"] : {
            path    = "/etc/firewalld/services/k0s-${role}.xml"
            content = file("${path.module}/k0s-${role}.firewalld-service.xml")
          }]

          runcmd = [
            "firewall-offline-cmd --add-service=k0s-controller",
            "firewall-offline-cmd --add-service=k0s-worker",
            "firewall-offline-cmd --add-masquerade",
            "systemctl reload firewalld.service",
          ]
        }))
      }

      worker = {
        user_data = format("#cloud-config\n%s", jsonencode({
          write_files = [{
            path    = "/etc/firewalld/services/k0s-worker.xml",
            content = file("${path.module}/k0s-worker.firewalld-service.xml"),
          }]

          runcmd = [
            "firewall-offline-cmd --add-service=k0s-worker",
            "firewall-offline-cmd --add-masquerade",
            "systemctl reload firewalld.service",
          ]
        }))
      }
    }
  }
}

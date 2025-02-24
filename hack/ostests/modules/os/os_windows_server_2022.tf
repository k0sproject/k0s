data "aws_ami" "windows_server_2022" {
  count = var.os == "windows_server_2022" ? 1 : 0

  owners      = ["801119661308"] # amazon
  name_regex  = "Windows_Server-2022-English-Full-Base-\\d+\\.\\d+\\.\\d+"
  most_recent = true

  filter {
    name   = "name"
    values = ["Windows_Server-2022-English-Full-Base-*"]
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
      error_message = "Unsupported architecture for Windows Server 2022."
    }
  }
}

locals {
  os_windows_server_2022 = var.os != "windows_server_2022" ? {} : {
    node_configs = {
      default    = local.os_alpine_3_20.node_configs.default
      controller = local.os_alpine_3_20.node_configs.controller

      worker = {
        ami_id        = one(data.aws_ami.windows_server_2022.*.id)
        instance_type = "t3a.medium"
        os_type       = "windows"
        volume        = { size = 50 }

        # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html#user-data-yaml-scripts
        # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2launch-v2-task-definitions.html#ec2launch-v2-enableopenssh
        # https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2launch-v2.html#ec2launch-v2-directory
        # C:\ProgramData\Amazon\EC2Launch\log\agent.log
        user_data = jsonencode({ version = 1.1, tasks = [{ task = "enableOpenSsh" }]})

        # Override the default Alpine ready script. Also checks SSH connectivity.
        ready_script = "whoami"

        connection = {
          type     = "ssh"
          username = "Administrator"
        }
      }
    }
  }
}

resource "terraform_data" "k0sctl_apply" {
  triggers_replace = [
    var.k0sctl_executable_path,
    sha256(jsonencode(local.k0sctl_config)),
  ]

  input = {
    hosts                    = var.hosts,
    ssh_private_key_filename = var.ssh_private_key_filename,
    k0sctl_config            = local.k0sctl_config,
  }

  provisioner "local-exec" {
    environment = {
      K0SCTL_EXECUTABLE_PATH = var.k0sctl_executable_path
      K0SCTL_CONFIG          = jsonencode(local.k0sctl_config)
    }

    command = <<-EOF
      printf %s "$K0SCTL_CONFIG" | env -u SSH_AUTH_SOCK SSH_KNOWN_HOSTS='' "$K0SCTL_EXECUTABLE_PATH" apply --disable-telemetry -c -
      EOF
  }
}

locals {
  num_controllers = length([for h in terraform_data.k0sctl_apply.output.hosts : h if h.is_controller])
  num_workers     = length([for h in terraform_data.k0sctl_apply.output.hosts : h if h.is_worker])
}

resource "terraform_data" "pre_flight_checks" {
  triggers_replace = [
    sha256(jsonencode(terraform_data.k0sctl_apply.output.k0sctl_config)),
    sha256(file("${path.module}/pre-flight-checks.sh")),
  ]

  input = {
    hosts                    = terraform_data.k0sctl_apply.output.hosts,
    ssh_private_key_filename = terraform_data.k0sctl_apply.output.ssh_private_key_filename,
    k0sctl_config            = terraform_data.k0sctl_apply.output.k0sctl_config,
  }

  connection {
    type        = terraform_data.k0sctl_apply.output.hosts[0].connection.type
    user        = terraform_data.k0sctl_apply.output.hosts[0].connection.username
    private_key = sensitive(file(terraform_data.k0sctl_apply.output.ssh_private_key_filename))
    host        = terraform_data.k0sctl_apply.output.hosts[0].ipv4
    agent       = false
  }

  provisioner "remote-exec" {
    inline = [
      "#!/usr/bin/env sh",
      format("set -- %d %d", local.num_controllers, local.num_workers),
      file("${path.module}/pre-flight-checks.sh"),
    ]
  }
}

data "external" "k0s_kubeconfig" {
  query = {
    k0sctl_config = jsonencode(local.k0sctl_config)
  }

  program = [
    "env", "sh", "-ec",
    <<-EOS
      jq '.k0sctl_config | fromjson' |
        { env -u SSH_AUTH_SOCK SSH_KNOWN_HOSTS='' "$1" kubeconfig --disable-telemetry -c - || echo ~~~FAIL; } |
        jq --raw-input --slurp "$2"
    EOS
    , "--",
    var.k0sctl_executable_path, <<-EOS
      if endswith("~~~FAIL\n") then
        error("Failed to generate kubeconfig!")
      else
        {kubeconfig: .}
      end
    EOS
  ]

  depends_on = [terraform_data.k0sctl_apply]
}

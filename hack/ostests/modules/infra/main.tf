resource "tls_private_key" "ssh" {
  # For Windows instances, AWS enforces RSA, and disallows ed25519 ¯\_(ツ)_/¯
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "ssh" {
  key_name   = "${var.resource_name_prefix}-ssh"
  public_key = tls_private_key.ssh.public_key_openssh
}

locals {
  default_node_config = merge ({
    os_type = "linux"
    volume  = { size = 20 }
  }, {
    x86_64 = { instance_type = "t3a.small" }
    arm64  = { instance_type = "t4g.small" }
  }[var.os.arch])

  node_role_templates = {
    controller = {
      data = {
        count         = var.controller_num_nodes
        is_controller = true, is_worker = false,
      }
      sources = [for s in [var.os.node_configs.controller, var.os.node_configs.default]: s if s != null]
    }

    "controller+worker" = {
      data = {
        count         = var.controller_worker_num_nodes
        is_controller = true, is_worker = true,
      }
      sources = [for s in [var.os.node_configs.controller_worker, var.os.node_configs.worker, var.os.node_configs.default]: s if s != null]
    }

    worker = {
      data = {
        count         = var.worker_num_nodes
        is_controller = false, is_worker = true,
      }
      sources = [for s in [var.os.node_configs.worker, var.os.node_configs.default]: s if s != null]
    }
  }

  node_roles = { for role, tmpl in local.node_role_templates: role => merge(tmpl.data, {
    node_config = {
      ami_id        =  coalesce(tmpl.sources.*.ami_id...)
      instance_type =  coalesce(concat(tmpl.sources, [local.default_node_config]).*.instance_type...)
      os_type       =  coalesce(concat(tmpl.sources, [local.default_node_config]).*.os_type...)
      volume        =  coalesce(concat(tmpl.sources, [local.default_node_config]).*.volume...)
      user_data     =  try(coalesce(tmpl.sources.*.user_data...), null)
      ready_script  =  try(coalesce(tmpl.sources.*.ready_script...), null)
      connection    =  coalesce(tmpl.sources.*.connection...)
    }
  })}

  nodes = merge([for role, params in local.node_roles : {
    for idx in range(params.count) : "${role}-${idx + 1}" => {
      role          = role
      is_controller = params.is_controller, is_worker = params.is_worker,
      node_config   = params.node_config,
    }
  }]...)
}

resource "aws_instance" "nodes" {
  for_each = local.nodes

  ami           = each.value.node_config.ami_id
  instance_type = each.value.node_config.instance_type
  subnet_id     = data.aws_subnet.default_for_selected_az.id

  user_data = each.value.node_config.user_data

  tags = {
    Name                              = "${var.resource_name_prefix}-${each.key}"
    "ostests.k0sproject.io/node-name" = each.key
    "k0sctl.k0sproject.io/host-role"  = each.value.role
  }

  key_name = aws_key_pair.ssh.key_name

  associate_public_ip_address = true
  source_dest_check           = !each.value.is_worker
  vpc_security_group_ids = concat(
    [aws_security_group.node.id],
    each.value.is_controller ? [aws_security_group.controller.id] : []
  )

  root_block_device {
    volume_type = "gp2"
    volume_size = each.value.node_config.volume.size
  }
}

resource "terraform_data" "ready_scripts" {
  for_each = { for name, params in local.nodes : name => params if params.node_config.ready_script != null }

  connection {
    type            = each.value.node_config.connection.type
    user            = each.value.node_config.connection.username
    target_platform = each.value.node_config.os_type == "windows" ? "windows" : "unix"
    private_key     = tls_private_key.ssh.private_key_pem
    host            = aws_instance.nodes[each.key].public_ip
    agent           = false
  }

  provisioner "remote-exec" {
    inline = [each.value.node_config.ready_script]
  }
}

resource "terraform_data" "provisioned_nodes" {
  depends_on = [terraform_data.ready_scripts]

  input = [for node in aws_instance.nodes : {
    name          = node.tags.Name,
    os_type       = local.nodes[node.tags["ostests.k0sproject.io/node-name"]].node_config.os_type,
    role          = node.tags["k0sctl.k0sproject.io/host-role"]
    is_controller = local.nodes[node.tags["ostests.k0sproject.io/node-name"]].is_controller
    is_worker     = local.nodes[node.tags["ostests.k0sproject.io/node-name"]].is_worker
    ipv4          = node.public_ip,
    connection    = local.nodes[node.tags["ostests.k0sproject.io/node-name"]].node_config.connection
  }]
}

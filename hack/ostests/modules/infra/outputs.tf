output "nodes" {
  value       = terraform_data.provisioned_nodes.output
  description = "The nodes that have been provisioned."
}

output "ssh_private_key" {
  value       = tls_private_key.ssh.private_key_openssh
  sensitive   = true
  description = "The private key that can be used to authenticate via SSH."
}

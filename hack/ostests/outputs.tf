output "hosts" {
  value       = var.k0sctl_skip ? local.hosts : module.k0sctl[0].hosts
  description = "The hosts that have been provisioned by k0sctl."
}

output "ssh_private_key_filename" {
  value       = var.k0sctl_skip ? local.ssh_private_key_filename : module.k0sctl[0].ssh_private_key_filename
  description = "The name of the private key file that has been used to authenticate via SSH."
}

output "k0sctl_config" {
  value       = var.k0sctl_skip ? null : module.k0sctl[0].k0sctl_config
  description = "The k0sctl config that has been used."
}

output "k0s_kubeconfig" {
  value       = var.k0sctl_skip ? null : module.k0sctl[0].k0s_kubeconfig
  description = "The k0s admin kubeconfig that may be used to connect to the cluster."
  sensitive   = true
}

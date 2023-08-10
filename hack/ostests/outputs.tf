output "hosts" {
  value       = module.k0sctl.hosts
  description = "The hosts that have been provisioned by k0sctl."
}

output "ssh_private_key_filename" {
  value       = module.k0sctl.ssh_private_key_filename
  description = "The name of the private key file that has been used to authenticate via SSH."
}

output "k0sctl_config" {
  value       = module.k0sctl.k0sctl_config
  description = "The k0sctl config that has been used."
}

output "k0s_kubeconfig" {
  value       = module.k0sctl.k0s_kubeconfig
  description = "The k0s admin kubeconfig that may be used to connect to the cluster."
  sensitive   = true
}

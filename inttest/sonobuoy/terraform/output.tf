output "controller_external_ip" {
  value = aws_eip.controller-ext.*.public_ip
}

output "worker_external_ip" {
  value = aws_instance.cluster-workers.*.public_ip
}

output "controller_count" {
  value = var.controller_count
}

output "controller_pem" {
  value     = local_file.aws_private_pem
  sensitive = true
}

output "cluster_name" {
  value = local.cluster_unique_identifier
}

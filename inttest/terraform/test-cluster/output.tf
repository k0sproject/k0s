output "controller_external_ip" {
  value = aws_eip.controller-ext.*.public_ip
}

output "worker_external_ip" {
  value = aws_instance.cluster-workers.*.public_ip
}

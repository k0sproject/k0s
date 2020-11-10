output "controller_external_ip" {
  value = aws_eip.controller-ext.*.public_ip
}

output "worker_external_ip" {
  value = aws_instance.cluster-workers.*.public_ip
}


output "controller_pem" {
  value = local_file.aws_private_pem
}

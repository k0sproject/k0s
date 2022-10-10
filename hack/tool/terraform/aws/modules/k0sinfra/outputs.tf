output "controllers" {
  value = [for host in aws_instance.controllers : host]
}

output "workers" {
  value = [for host in aws_instance.workers : host]
}

output "loadbalancer_dns" {
  value = aws_lb.controllers.dns_name
}
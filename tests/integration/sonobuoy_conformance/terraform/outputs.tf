output "controller_external_ip" {
  value = module.k0s-sonobuoy.controller_external_ip
}

output "worker_external_ip" {
  value = module.k0s-sonobuoy.worker_external_ip
}


output "controller_pem" {
  value     = module.k0s-sonobuoy.controller_pem.content
  sensitive = true
}
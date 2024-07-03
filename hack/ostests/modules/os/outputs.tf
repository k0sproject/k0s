output "os" {
  value       = merge(local.os[var.os], { arch = var.arch })
  description = "The OS confguration."
}

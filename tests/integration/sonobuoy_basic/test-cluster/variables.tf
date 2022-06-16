variable "cluster_name" {
  type = string
}

variable "controller_count" {
  type    = number
  default = 1
}

variable "worker_count" {
  type    = number
  default = 2
}

variable "cluster_flavor" {
  type    = string
  default = "c4.xlarge"
}

variable "k0s_version" {
  type        = string
  description = "The k0s version to deploy on the machines. May be an exact version, \"stable\" or \"latest\"."
  default     = "stable"
}

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

variable "instance_type" {
  type    = string
  default = "c4.xlarge"
}

variable "instance_arch" {
  type    = string
  default = "amd64"
}

variable "k0s_version" {
  type        = string
  description = <<EOF
  The k0s version to deploy on the machines. May be an exact version, \"stable\" or \"latest\".
  EOF
  default     = ""
}

variable "k0s_binary_path" {
  type    = string
  default = ""
}

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

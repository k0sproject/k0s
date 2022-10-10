variable "vpc_id" {
  description = "The identifier of the VPC that should be used"
  type        = string
}

variable "subnet_id" {
  description = "The ID of the subnet that will be used for networking"
  type        = string
}

variable "name" {
  description = "The 'friendly' name of the cluster"
  type        = string
}

variable "controllers" {
  description = "The number of k0s controllers"
  type        = number
}

variable "workers" {
  description = "The number of k0s workers"
  type        = number
}

variable "instance_type" {
  description = "The type of EC2 instances to create"
  type        = string
  default     = "c5a.xlarge"
}

variable "k0s_binary" {
  description = "The name of the k0s binary in '/tool/data' that will be uploaded to nodes"
  type        = string
}

variable "k0s_version" {
  description = "The version of the k0s binary"
  type        = string
}
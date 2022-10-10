variable "region" {
  description = "The AWS region where resources will live"
  type        = string
}

variable "name" {
  description = "The name of the VPC"
  type        = string
}

variable "cidr" {
  description = "The CIDR block for the VPC (/16)"
  type        = string
  default     = "10.0.0.0/16"
}

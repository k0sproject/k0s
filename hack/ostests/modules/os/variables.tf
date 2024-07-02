variable "os" {
  type        = string
  description = "The OS which is to be configured."
}

variable "arch" {
  type        = string
  description = "The architecture for the OS."
}

variable "additional_ingress_cidrs" {
  type        = list(string)
  nullable    = false
  description = "Additional CIDRs that are allowed for ingress traffic."
  default     = []
}

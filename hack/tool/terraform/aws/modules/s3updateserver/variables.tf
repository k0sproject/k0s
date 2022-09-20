variable "name" {
  description = "The name of the S3 bucket acting as an update server"
  type        = string
}

variable "region" {
  description = "The AWS region where resources will live"
  type        = string
}

variable "k0s_binary" {
  description = "The k0s binary that is considered an update"
  type        = string
}

variable "k0s_airgap_bundle" {
  description = "The k0s airgap bundle is considered an update"
  type        = string
  default     = ""
}

variable "expiration_seconds" {
  description = "How long the presigned URL lasts (in seconds)"
  type        = number

  # default = 30 minutes
  default = 1800
}
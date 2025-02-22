variable "resource_name_prefix" {
  type        = string
  description = "Prefix to be prepended to all resource names."

  validation {
    condition     = var.resource_name_prefix != null && can(regex("^([a-z][a-z0-9-_]*)?$", var.resource_name_prefix))
    error_message = "Invalid resource name prefix."
  }
}

variable "os" {
  type = object({
    arch = string
    node_configs = object({
      default = object({
        ami_id        = string
        instance_type = optional(string)
        os_type       = optional(string)

        volume = optional(object({
          size = number
        }))

        user_data    = optional(string)
        ready_script = optional(string)

        connection = object({
          type     = string
          username = string
        })
      })

      controller = optional(object({
        ami_id        = optional(string)
        instance_type = optional(string)
        os_type       = optional(string)

        volume = optional(object({
          size = number
        }))

        user_data    = optional(string)
        ready_script = optional(string)

        connection = optional(object({
          type     = string
          username = string
        }))
      }))

      controller_worker = optional(object({
        ami_id        = optional(string)
        instance_type = optional(string)
        os_type       = optional(string)

        volume = optional(object({
          size = number
        }))

        user_data    = optional(string)
        ready_script = optional(string)

        connection = optional(object({
          type     = string
          username = string
        }))
      }))

      worker = optional(object({
        ami_id        = optional(string)
        instance_type = optional(string)
        os_type       = optional(string)

        volume = optional(object({
          size = number
        }))

        user_data    = optional(string)
        ready_script = optional(string)

        connection = optional(object({
          type     = string
          username = string
        }))
      }))
    })
  })

  description = "The OS configuration."

  validation {
    condition     = contains(["x86_64", "arm64"], var.os.arch)
    error_message = "Invalid processor architecture."
  }
}

variable "additional_ingress_cidrs" {
  type        = list(string)
  nullable    = false
  description = "Additional CIDRs that are allowed for ingress traffic."
  default     = []
}

variable "controller_num_nodes" {
  type        = number
  description = "The number controller nodes to spin up."
  default     = 3 # Test an HA cluster by default
}

variable "controller_worker_num_nodes" {
  type        = number
  description = "The number controller+worker nodes to spin up."
  default     = 0
}

variable "worker_num_nodes" {
  type        = number
  description = "The number worker nodes to spin up."
  default     = 2 # That's the minimum for conformance tests
}

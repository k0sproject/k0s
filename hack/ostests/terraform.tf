terraform {
  required_version = ">= 1.8.0"

  required_providers {
    aws    = { source = "hashicorp/aws", version = "~> 5.0", }
    local  = { source = "opentffoundation/local", version = "~> 2.0", }
    random = { source = "hashicorp/random", version = "~> 3.0", }
  }
}

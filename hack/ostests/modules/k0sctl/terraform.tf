terraform {
  required_version = ">= 1.8.0"

  required_providers {
    external = { source = "hashicorp/external", version = "~> 2.0", }
    local    = { source = "opentffoundation/local", version = "~> 2.0", }
    http     = { source = "hashicorp/http", version = "~> 3.0", }
  }
}

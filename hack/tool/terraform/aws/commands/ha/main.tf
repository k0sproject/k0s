terraform {
  required_version = ">= 1.2.9"
  backend "local" {}

  required_providers {
    local = {
      source = "hashicorp/local"
    }
  }
}

provider "aws" {
  region = var.region
}

module "privatekey" {
  source  = "/tool/terraform/scripts/aws/modules/privatekey"
  name    = var.name
  outfile = "/tool/data/private.pem"
}

data "aws_vpc" "default" {
  default = true
}

data "aws_availability_zones" "this" {
  state = "available"
}

data "aws_subnet" "this" {
  availability_zone = data.aws_availability_zones.this.names[0]
  default_for_az    = true
  vpc_id            = data.aws_vpc.default.id
}

module "k0sinfra" {
  source        = "/tool/terraform/scripts/aws/modules/k0sinfra"
  vpc_id        = data.aws_vpc.default.id
  subnet_id     = data.aws_subnet.this.id
  name          = var.name
  controllers   = var.controllers
  workers       = var.workers
  instance_type = var.instance_type
  k0s_binary    = var.k0s_binary
  k0s_version   = var.k0s_version

  # TODO: add a variable for the private key name
  depends_on = [
    module.privatekey
  ]
}

# If a k0s binary update has been provided, it will be hosted in an S3 bucket, referenced
# using presigned URLs that have an expiration.
module "s3updateserver" {
  count = var.k0s_update_binary != "" ? 1 : 0

  source            = "/tool/terraform/scripts/aws/modules/s3updateserver"
  name              = var.name
  region            = var.region
  k0s_binary        = format("/tool/data/%s", var.k0s_update_binary)
  k0s_airgap_bundle = var.k0s_update_airgap_bundle != "" ? format("/tool/data/%s", var.k0s_update_airgap_bundle) : ""
}

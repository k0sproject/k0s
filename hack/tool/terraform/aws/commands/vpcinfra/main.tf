terraform {
  required_version = ">= 1.2.9"
  backend "local" {}
}

provider "aws" {
  region = var.region
}

module "vpcinfra" {
  source = "/tool/terraform/scripts/aws/modules/vpcinfra"
  name   = var.name
  region = var.region
  cidr   = var.cidr
}

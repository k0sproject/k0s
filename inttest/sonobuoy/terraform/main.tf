provider "aws" {
  region = "eu-west-1"
}

resource "random_id" "cluster_identifier" {
  byte_length = 4
}

locals {
  cluster_unique_identifier = format("%s-%s", var.cluster_name, random_id.cluster_identifier.hex)
}

resource "tls_private_key" "k8s-conformance-key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "cluster-key" {
  key_name   = format("%s_key", local.cluster_unique_identifier)
  public_key = tls_private_key.k8s-conformance-key.public_key_openssh
}

// Save the private key to filesystem
resource "local_file" "aws_private_pem" {
  file_permission = "600"
  filename        = format("%s/%s", path.module, "aws_private.pem")
  content         = tls_private_key.k8s-conformance-key.private_key_pem
}

resource "aws_security_group" "cluster_allow_ssh" {
  name        = format("%s-allow-ssh", local.cluster_unique_identifier)
  description = "Allow ssh inbound traffic"
  vpc_id      = aws_vpc.cluster-vpc.id

  // Allow all incoming and outgoing ports.
  // TODO: need to create a more restrictive policy
  ingress {
    description = "SSH from VPC"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = format("%s-allow-ssh", local.cluster_unique_identifier)
  }
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = [format("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-%s-server-*", var.instance_arch)]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"]
}

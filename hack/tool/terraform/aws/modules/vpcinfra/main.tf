resource "aws_vpc" "vpc" {
  cidr_block           = var.cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = var.name
  }
}

resource "aws_default_security_group" "default" {
  vpc_id = aws_vpc.vpc.id

  tags = {
    Name = var.name
  }
}

resource "aws_security_group_rule" "ingress_allow_all" {
  description       = "allow all"
  type              = "ingress"
  from_port         = 0
  to_port           = 0
  protocol          = "all"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_default_security_group.default.id
}

resource "aws_security_group_rule" "egress_allow_all" {
  description       = "allow all"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "all"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_default_security_group.default.id
}

resource "aws_internet_gateway" "gateway" {
  vpc_id = aws_vpc.vpc.id
  tags = {
    Name = var.name
  }
}

resource "aws_route_table" "routing" {
  vpc_id = aws_vpc.vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.gateway.id
  }
  tags = {
    Name = var.name
  }
}

resource "aws_main_route_table_association" "mrta" {
  vpc_id         = aws_vpc.vpc.id
  route_table_id = aws_route_table.routing.id
}

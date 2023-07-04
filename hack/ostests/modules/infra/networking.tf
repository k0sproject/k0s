data "aws_vpc" "default" {
  default = true
}

data "aws_subnet" "default_for_selected_az" {
  vpc_id            = data.aws_vpc.default.id
  availability_zone = one(random_shuffle.selected_az.result)
  default_for_az    = true
}

resource "aws_route_table_association" "default_for_selected_az" {
  subnet_id      = data.aws_subnet.default_for_selected_az.id
  route_table_id = data.aws_vpc.default.main_route_table_id
}

resource "aws_security_group" "node" {
  name        = "${var.resource_name_prefix}-node"
  description = "Allow ALL ingress traffic inside the subnet, ALL egress traffic to the outside and SSH from the internet."
  vpc_id      = data.aws_subnet.default_for_selected_az.vpc_id

  tags = {
    Name = "${var.resource_name_prefix}-node"
  }
}

resource "aws_security_group_rule" "node_subnet_ingress" {
  description       = "Allow ALL ingress traffic inside the subnet."
  security_group_id = aws_security_group.node.id
  type              = "ingress"
  from_port         = 0
  to_port           = 65535
  protocol          = "all"
  cidr_blocks       = [data.aws_subnet.default_for_selected_az.cidr_block]
}

resource "aws_security_group_rule" "node_additional_ingress" {
  count = length(var.additional_ingress_cidrs) > 0 ? 1 : 0

  description       = "Allow ingress from additional CIDRs."
  security_group_id = aws_security_group.node.id
  type              = "ingress"
  from_port         = 0
  to_port           = 65535
  protocol          = "all"
  cidr_blocks       = var.additional_ingress_cidrs
}

resource "aws_security_group_rule" "node_all_egress" {
  description       = "Allow ALL egress traffic."
  security_group_id = aws_security_group.node.id
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "all"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "node_public_ssh" {
  description       = "Allow SSH access from the internet."
  security_group_id = aws_security_group.node.id
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group" "controller" {
  name        = "${var.resource_name_prefix}-controller"
  description = "Allow API server access from the internet."
  vpc_id      = data.aws_subnet.default_for_selected_az.vpc_id

  tags = {
    Name = "${var.resource_name_prefix}-controller"
  }
}

resource "aws_security_group_rule" "public_api_server" {
  description       = "Allow API server access from the internet."
  security_group_id = aws_security_group.controller.id
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
}

data "aws_key_pair" "cluster" {
  key_name = var.name
}

# Find the latest Canonical Ubuntu 22.04 image
data "aws_ami" "ubuntu" {
  name_regex  = "ubuntu-jammy"
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  # Owner = Canonical
  owners = ["099720109477"]
}

data "aws_vpc" "this" {
  id = var.vpc_id
}

data "aws_subnet" "this" {
  id = var.subnet_id
}

resource "aws_security_group" "this" {
  name   = var.name
  vpc_id = data.aws_vpc.this.id

  tags = {
    Name = var.name
  }
}

# SSH

resource "aws_security_group_rule" "ingress_allow_all_external" {
  description       = "Allow ALL traffic from the outside"
  security_group_id = aws_security_group.this.id
  type              = "ingress"
  from_port         = 0
  to_port           = 65535
  protocol          = "all"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "egress_allow_all_external" {
  description       = "Allow ALL traffic to the outside"
  security_group_id = aws_security_group.this.id
  type              = "egress"
  from_port         = 0
  to_port           = 65335
  protocol          = "all"
  cidr_blocks       = ["0.0.0.0/0"]
}

resource "aws_route_table_association" "this" {
  subnet_id      = data.aws_subnet.this.id
  route_table_id = data.aws_vpc.this.main_route_table_id
}

resource "aws_instance" "controllers" {
  count         = var.controllers
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.instance_type
  subnet_id     = data.aws_subnet.this.id

  tags = {
    Name    = format("%s-controller%d", var.name, count.index)
    Role    = "controller"
    Project = "K0S"
  }

  key_name                    = data.aws_key_pair.cluster.key_name
  vpc_security_group_ids      = [aws_security_group.this.id]
  associate_public_ip_address = true

  root_block_device {
    # TODO: don't hardcode these
    volume_type = "gp2"
    volume_size = 20
  }
}

# Load Balancer

resource "aws_lb" "controllers" {
  name               = format("%s-lb", var.name)
  internal           = false
  load_balancer_type = "network"
  subnets            = [data.aws_subnet.this.id]
}

# Kubernetes API listener + group

resource "aws_lb_listener" "p6443" {
  load_balancer_arn = aws_lb.controllers.arn
  port              = aws_lb_target_group.p6443.port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.p6443.arn
  }
}

resource "aws_lb_target_group" "p6443" {
  name     = format("%s-lbtg-kubeapi", var.name)
  port     = 6443
  protocol = "TCP"
  vpc_id   = data.aws_vpc.this.id
}

resource "aws_lb_target_group_attachment" "p6443" {
  count            = var.controllers
  target_group_arn = aws_lb_target_group.p6443.arn
  target_id        = aws_instance.controllers[count.index].id
  port             = aws_lb_target_group.p6443.port
}

# K0S API listener + group

resource "aws_lb_listener" "p9443" {
  load_balancer_arn = aws_lb.controllers.arn
  port              = aws_lb_target_group.p9443.port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.p9443.arn
  }
}

resource "aws_lb_target_group" "p9443" {
  name     = format("%s-lbtg-k0sapi", var.name)
  port     = 9443
  protocol = "TCP"
  vpc_id   = data.aws_vpc.this.id
}

resource "aws_lb_target_group_attachment" "p9443" {
  count            = var.controllers
  target_group_arn = aws_lb_target_group.p9443.arn
  target_id        = aws_instance.controllers[count.index].id
  port             = aws_lb_target_group.p9443.port
}

# Konnectivity Agent listener + group

resource "aws_lb_listener" "p8132" {
  load_balancer_arn = aws_lb.controllers.arn
  port              = aws_lb_target_group.p8132.port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.p8132.arn
  }
}

resource "aws_lb_target_group" "p8132" {
  name     = format("%s-lbtg-konnagent", var.name)
  port     = 8132
  protocol = "TCP"
  vpc_id   = data.aws_vpc.this.id
}

resource "aws_lb_target_group_attachment" "p8132" {
  count            = var.controllers
  target_group_arn = aws_lb_target_group.p8132.arn
  target_id        = aws_instance.controllers[count.index].id
  port             = aws_lb_target_group.p8132.port
}

# Konnectivity Admin + group

resource "aws_lb_listener" "p8133" {
  load_balancer_arn = aws_lb.controllers.arn
  port              = aws_lb_target_group.p8133.port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.p8133.arn
  }
}

resource "aws_lb_target_group" "p8133" {
  name     = format("%s-lbtg-konnadmin", var.name)
  port     = 8133
  protocol = "TCP"
  vpc_id   = data.aws_vpc.this.id
}

resource "aws_lb_target_group_attachment" "p8133" {
  count            = var.controllers
  target_group_arn = aws_lb_target_group.p8133.arn
  target_id        = aws_instance.controllers[count.index].id
  port             = aws_lb_target_group.p8133.port
}

resource "aws_instance" "workers" {
  count         = var.workers
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.instance_type
  subnet_id     = data.aws_subnet.this.id

  key_name                    = data.aws_key_pair.cluster.key_name
  vpc_security_group_ids      = [aws_security_group.this.id]
  associate_public_ip_address = true
  source_dest_check           = false


  root_block_device {
    # TODO: don't hardcode this
    volume_type = "gp2"
    volume_size = 20
  }

  tags = {
    Name    = format("%s-worker%d", var.name, count.index)
    Role    = "worker"
    Project = "K0S"
  }
}

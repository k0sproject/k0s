resource "aws_vpc" "cluster-vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true
  tags = {
    Name = format("%s-vpc", local.cluster_unique_identifier)
  }
}

resource "aws_subnet" "cluster-subnet" {
  cidr_block        = cidrsubnet(aws_vpc.cluster-vpc.cidr_block, 3, 1)
  vpc_id            = aws_vpc.cluster-vpc.id
  availability_zone = "eu-west-1a"
  tags = {
    Name = format("%s-subnet", local.cluster_unique_identifier)
  }
}

resource "aws_internet_gateway" "cluster-gw" {
  vpc_id = aws_vpc.cluster-vpc.id
  tags = {
    Name = format("%s-gw", local.cluster_unique_identifier)
  }
}

resource "aws_route_table" "route-table-cluster-test-env" {
  vpc_id = aws_vpc.cluster-vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.cluster-gw.id
  }
  tags = {
    Name = format("%s-route-table", local.cluster_unique_identifier)
  }
}

resource "aws_route_table_association" "cluster-subnet-association" {
  subnet_id      = aws_subnet.cluster-subnet.id
  route_table_id = aws_route_table.route-table-cluster-test-env.id
}

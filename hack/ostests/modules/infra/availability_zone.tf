data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ec2_instance_type_offerings" "in_available_azs" {
  for_each = toset([for node in local.nodes : node.node_config.instance_type])

  filter {
    name   = "instance-type"
    values = [each.value]
  }

  filter {
    name   = "location"
    values = toset(data.aws_availability_zones.available.names)
  }

  location_type = "availability-zone"
}

resource "random_shuffle" "selected_az" {
  # Select a single availability zone that is available *and* offers all the required instance types.
  input        = setintersection([for k, v in data.aws_ec2_instance_type_offerings.in_available_azs : v.locations]...)
  result_count = 1
}

resource "tls_private_key" "this" {
  algorithm = "ED25519"
}

resource "aws_key_pair" "this" {
  key_name   = var.name
  public_key = tls_private_key.this.public_key_openssh
}

# Save the private key locally
resource "local_file" "this" {
  file_permission = 600
  filename        = var.outfile
  content         = tls_private_key.this.private_key_openssh
}
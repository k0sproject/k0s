resource "random_id" "this" {
  byte_length = 8
}

locals {
  name = format("%s-s3-%s", var.name, random_id.this.hex)
}

resource "aws_s3_bucket" "files" {
  bucket = local.name
}

resource "aws_s3_bucket_acl" "files" {
  bucket = aws_s3_bucket.files.id
  acl    = "private"
}

resource "aws_s3_object" "k0s" {
  bucket = aws_s3_bucket.files.id
  key    = basename(var.k0s_binary)
  source = var.k0s_binary
  etag   = filemd5(var.k0s_binary)
  acl    = "private"
}

resource "aws_s3_object" "k0s_airgap_bundle" {
  count  = var.k0s_airgap_bundle != "" ? 1 : 0
  bucket = aws_s3_bucket.files.id
  key    = basename(var.k0s_airgap_bundle)
  source = var.k0s_airgap_bundle
  # etag   = filemd5(var.k0s_airgap_bundle)
  acl = "private"
}

# Manually create a presigned URL of the k0s binary in the bucket. This URL will expire
# after a period of time.
resource "null_resource" "k0s" {
  depends_on = [
    aws_s3_object.k0s
  ]

  provisioner "local-exec" {
    command = "printf '%s' $(/usr/local/bin/aws s3 presign --expires-in ${var.expiration_seconds} --region ${var.region} s3://${aws_s3_bucket.files.bucket}/${aws_s3_object.k0s.key}) > /tool/data/k0s_update_binary.presigned_url"
  }
}

resource "null_resource" "k0s_airgap_bundle" {
  count = var.k0s_airgap_bundle != "" ? 1 : 0

  depends_on = [
    aws_s3_object.k0s_airgap_bundle[0]
  ]

  provisioner "local-exec" {
    command = "printf '%s' $(/usr/local/bin/aws s3 presign --expires-in ${var.expiration_seconds} --region ${var.region} s3://${aws_s3_bucket.files.bucket}/${aws_s3_object.k0s_airgap_bundle[0].key}) > /tool/data/k0s_airgap_bundle.presigned_url"
  }
}

data "local_file" "k0s_update_binary_presigned_url" {
  depends_on = [
    null_resource.k0s
  ]

  filename = "/tool/data/k0s_update_binary.presigned_url"
}

data "local_file" "k0s_airgap_bundle_presigned_url" {
  count = var.k0s_airgap_bundle != "" ? 1 : 0

  depends_on = [
    null_resource.k0s_airgap_bundle
  ]

  filename = "/tool/data/k0s_airgap_bundle.presigned_url"
}
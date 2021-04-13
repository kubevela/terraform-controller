resource "aws_s3_bucket" "bucket-acl" {
  bucket = var.bucket
  acl    = var.acl
}

output "BUCKET_NAME" {
  value = aws_s3_bucket.bucket-acl.bucket_domain_name
}

variable "bucket" {
  default = "vela-website"
}

variable "acl" {
  default = "private"
}

resource "aws_s3_bucket" "bucket-acl" {
  bucket = var.bucket
  acl    = var.acl
}

output "RESOURCE_IDENTIFIER" {
  description = "The identifier of the resource"
  value       = aws_s3_bucket.bucket-acl.bucket_domain_name
}

output "BUCKET_NAME" {
  value       = aws_s3_bucket.bucket-acl.bucket_domain_name
  description = "The name of the S3 bucket"
}

variable "bucket" {
  description = "S3 bucket name"
  default     = "vela-website-2022-0614"
  type        = string
}

variable "acl" {
  description = "S3 bucket ACL"
  default     = "private"
  type        = string
}

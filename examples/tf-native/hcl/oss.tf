resource "alicloud_oss_bucket" "bucket-acl" {
  bucket = var.bucket
  acl    = var.acl
}

output "BUCKET_NAME" {
  value = "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
}

variable "bucket" {
  default = "vela-website"
}

variable "acl" {
  default = "private"
}
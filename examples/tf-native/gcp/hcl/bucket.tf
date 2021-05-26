resource "google_storage_bucket" "bucket" {
  name = var.bucket
}

output "BUCKET_URL" {
  value = google_storage_bucket.bucket.url
}

variable "bucket" {
  default = "vela-website"
}

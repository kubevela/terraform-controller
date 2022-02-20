resource "google_storage_bucket" "bucket" {
  name     = var.bucket
  location = var.location
}

output "BUCKET_URL" {
  value = google_storage_bucket.bucket.url
  description = "Bucket URL"
}

variable "bucket" {
  description = "The name of the storage bucket to create"
  type        = "string"
}

variable "location" {
  description = "The location of the storage bucket to create"
  type        = "string"
}
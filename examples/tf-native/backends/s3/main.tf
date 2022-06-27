terraform {
  backend "s3" {
    bucket = "tf-state-poc-0608"
    key    = "sss"
    region = "us-east-1"
  }
}


resource "random_id" "server" {
  byte_length = 8
}

output "random_id" {
  value = random_id.server.hex
}

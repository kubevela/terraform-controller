terraform {
  required_providers {
    tencentcloud = {
      source = "tencentcloudstack/tencentcloud"
    }
  }
}

resource "tencentcloud_vpc" "vpc" {
  name         = var.vpc_name
  cidr_block   = var.vpc_cidr
  is_multicast = var.vpc_is_multicast
  dns_servers  = length(var.vpc_dns_servers) > 0 ? var.vpc_dns_servers : null
  tags         = var.vpc_tags
}

variable "vpc_name" {
  description = "The vpc name used to launch a new vpc when 'vpc_id' is not specified."
  default     = "tf-modules-vpc"
}

variable "vpc_cidr" {
  description = "The cidr block used to launch a new vpc when 'vpc_id' is not specified."
  default     = "172.16.0.0/16"
}

variable "vpc_is_multicast" {
  description = "Specify the vpc is multicast when 'vpc_id' is not specified."
  default     = true
}

variable "vpc_dns_servers" {
  description = "Specify the vpc dns servers when 'vpc_id' is not specified."
  type        = list(string)
  default     = []
}

variable "vpc_tags" {
  description = "Additional tags for the vpc."
  type        = map(string)
  default     = {}
}

output "VPC_ID" {
  description = "The id of vpc."
  value       = tencentcloud_vpc.vpc.id
}
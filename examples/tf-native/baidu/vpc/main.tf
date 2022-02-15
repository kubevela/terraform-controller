    terraform {
      required_providers {
        baiducloud = {
          source  = "baidubce/baiducloud"
          version = "1.12.0"
        }
      }
    }

    resource "baiducloud_vpc" "default" {
      name        = var.name
      description = var.description
      cidr        = var.cidr
    }

    variable "name" {
      default     = "terraform-vpc"
      description = "The name of the VPC"
      type        = string
    }

    variable "description" {
      description = "The description of the VPC"
      default     = "this is created by terraform"
      type        = string
    }

    variable "cidr" {
      description = "The CIDR of the VPC"
      default     = "192.168.0.0/24"
      type        = string
    }

    output "vpcs" {
      value = baiducloud_vpc.default.id
    }
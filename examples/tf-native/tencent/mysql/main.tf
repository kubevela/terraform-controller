terraform {
  required_providers {
    tencentcloud = {
      source = "tencentcloudstack/tencentcloud"
    }
  }
}

resource "tencentcloud_mysql_instance" "main" {
  instance_name     = var.instance_name
  root_password     = var.instance_password
  mem_size          = var.mem_size
  volume_size       = var.volume_size
  engine_version    = "5.7"

  parameters = {
    max_connections = "1000"
  }
}

resource "tencentcloud_mysql_account" "mysql_account" {
  mysql_id    = tencentcloud_mysql_instance.main.id
  name        = var.account_name
  password    = var.account_password
  description = "for test"
}

output "DB_NAME" {
  value = var.instance_name
}
output "DB_USER" {
  value = var.account_name
}
output "DB_PASSWORD" {
  value = var.account_password
}
output "DB_HOST" {
  value = tencentcloud_mysql_instance.main.internet_host
}
output "DB_PORT" {
  value = tencentcloud_mysql_instance.main.internet_port
}

variable "instance_name" {
  description = "mysql instance name"
  type = string
  default = "poc"
}

variable "instance_password" {
  description = "mysql instance root password"
  type = string
  default = "test1234"
}

variable "mem_size" {
  description = "mysql instance memory size"
  type = number
  default = 1000
}

variable "volume_size" {
  description = "mysql instance volume size"
  type = number
  default = 50
}

variable "account_name" {
  description = "mysql account name"
  type = string
  default = "test"
}

variable "account_password" {
  description = "mysql account password"
  type = string
  default = "test1234"
}

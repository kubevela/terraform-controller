resource "tencentcloud_redis_instance" "main" {
  type              = "master_slave_redis"
  availability_zone = var.availability_zone
  name              = var.instance_name
  password          = var.user_password
  mem_size          = var.mem_size
  port              = var.port
}

output "DB_IP" {
  value = tencentcloud_redis_instance.main.ip
}

output "DB_PASSWORD" {
  value = var.user_password
}

output "DB_PORT" {
  value = var.port
}

variable "availability_zone" {
  description = "The available zone ID of an instance to be created."
  type = string
}

variable "instance_name" {
  description = "redis instance name"
  type = string
  default = "poc"
}

variable "user_password" {
  description = "redis instance password"
  type = string
  default = "test1234"
}

variable "mem_size" {
  description = "redis instance memory size"
  type = number
  default = 1000
}

variable "port" {
  description = "The port used to access a redis instance."
  type = number
  default = 6379
}

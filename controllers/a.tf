variable "aaa" {
  type = list(string)
}

output "out" {
  value = var.aaa
}
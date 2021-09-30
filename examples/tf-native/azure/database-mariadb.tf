# Configure the Microsoft Azure Provider
provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "example" {
  name = "tfex-mariadb-database-RG"
  location = "West Europe"
}

resource "azurerm_mariadb_server" "example" {
  name = "mariadb-svr-sample"
  location = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name

  sku_name = "B_Gen5_2"

  storage_mb = 51200
  backup_retention_days = 7
  geo_redundant_backup_enabled = false

  administrator_login = var.username
  administrator_login_password = var.password
  version = "10.2"
  ssl_enforcement_enabled = true
}

resource "azurerm_mariadb_database" "example" {
  name = var.name
  resource_group_name = azurerm_resource_group.example.name
  server_name = azurerm_mariadb_server.example.name
  charset = "utf8"
  collation = "utf8_general_ci"
}

variable "name" {
  default = "mariadb_database"
  type = string
  description = "Database instance name"
}

variable "username" {
  default = "acctestun"
  type = string
  description = "Database instance username"
}

variable "password" {
  default = "H@Sh1CoR3!faked"
  type = string
  description = "Database instance password"
}

############################
# Deploying vSphere Folder #
############################

provider "vsphere" {
  user           = var.vsphere-user
  password       = var.vsphere-password
  vsphere_server = var.vsphere-server

  # If you have a self-signed cert
  allow_unverified_ssl = var.vsphere-unverified-ssl
}

#############
# Variables #
#############

variable "vsphere-user" {
  type        = string
  description = "VMware vSphere user name"
}

variable "vsphere-password" {
  type        = string
  description = "VMware vSphere password"
}

variable "vsphere-server" {
  type        = string
  description = "VMware vCenter server FQDN / IP"
}

variable "vsphere-unverified-ssl" {
  type        = bool
  description = "Is the VMware vCenter using a self signed certificated (true/false)"
  default     = true
}

variable "vsphere-datacenter" {
  type        = string
  description = "VMware vSphere datacenter"
}

variable "folder-name" {
  type        = string
  description = "The name of folder"
}

variable "folder-type" {
  type        = string
  description = "The type of folder"
}

##########
# Folder #
##########

data "vsphere_datacenter" "dc" {
  name = var.vsphere-datacenter
}

resource "vsphere_folder" "folder" {
  path          = var.folder-name
  type          = var.folder-type
  datacenter_id = data.vsphere_datacenter.dc.id
}

output "folder" {
    value       = "folder-${var.folder-type}-${var.folder-name}"
}
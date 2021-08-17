terraform {
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.2.1"
    }
  }
}

data "ec_stack" "latest" {
  version_regex = "latest"
  region        = var.ec_region
}

resource "ec_deployment" "project" {
  name = var.project_name

  region                 = var.ec_region
  version                 = data.ec_stack.latest.version
  deployment_template_id = "gcp-io-optimized"

  elasticsearch {
    autoscale = "true"
  }

  kibana {}
}

output "ES_HTTPS_ENDPOINT" {
  value = ec_deployment.project.elasticsearch[0].https_endpoint
}

output "ES_PASSWORD" {
  value = ec_deployment.project.elasticsearch_password
  sensitive = true
}

variable "ec_region" {
  default = "gcp-us-west1"
}

variable "project_name" {
  default = "example"
}

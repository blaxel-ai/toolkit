terraform {
  required_providers {
    blaxel = {
      source = "blaxel-ai/blaxel"
    }
  }
}

provider "blaxel" {
  # Credentials from BL_API_KEY and BL_WORKSPACE env vars
}

# Sandbox cluster - deploys a template and then N sandboxes from it
resource "blaxel_sandbox_cluster" "example" {
  name                = "my-cluster"
  template_sandbox    = "my-template"
  template_image      = "blaxel/prod-base:latest"
  replicas            = 3
  sandbox_name_prefix = "worker"
  region              = "us-pdx-1"
  memory              = 4096
  generation          = "mk3"
  enabled             = true
}

output "deployed_sandboxes" {
  value = blaxel_sandbox_cluster.example.deployed_sandboxes
}

output "cluster_id" {
  value = blaxel_sandbox_cluster.example.id
}
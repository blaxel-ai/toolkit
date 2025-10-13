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

# Single sandbox resource
resource "blaxel_sandbox" "example" {
  name         = "my-sandbox"
  display_name = "My Example Sandbox"
  image        = "blaxel/prod-base:latest"
  region       = "us-pdx-1"
  memory       = 4096
  generation   = "mk3"
  enabled      = true
}

output "sandbox_status" {
  value = blaxel_sandbox.example.status
}

output "sandbox_id" {
  value = blaxel_sandbox.example.id
}


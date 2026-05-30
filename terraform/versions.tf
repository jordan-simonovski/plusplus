terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # Partial backend config. Real values live in backend.hcl (committed) and are
  # passed via: terraform init -backend-config=backend.hcl
  backend "s3" {}
}

terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # Local state on purpose: bootstrap creates the very S3 bucket the main config
  # uses for remote state, so it cannot use that bucket itself (chicken/egg).
  # Run this once, locally, with admin credentials. The state file is gitignored.
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = "plusplus"
      ManagedBy = "terraform-bootstrap"
    }
  }
}

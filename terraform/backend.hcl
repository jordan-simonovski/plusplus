# Backend config for `terraform init -backend-config=backend.hcl`.
# Set `bucket` to the value output by the bootstrap config (state_bucket_name).
# This file contains no secrets and is safe to commit.
bucket       = "plusplus-karma-bucket-prod"
key          = "plusplus/terraform.tfstate"
region       = "ap-southeast-2"
encrypt      = true
use_lockfile = true

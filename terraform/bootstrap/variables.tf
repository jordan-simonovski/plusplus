variable "aws_region" {
  description = "AWS region for state bucket and IAM."
  type        = string
  default     = "ap-southeast-2"
}

variable "state_bucket_name" {
  description = "Globally-unique S3 bucket name for Terraform remote state."
  type        = string
  default     = "plusplus-karma-bucket-prod"
}

variable "github_repository" {
  description = "GitHub repo allowed to assume the CI role, as owner/name."
  type        = string
  default     = "jordan-simonovski/plusplus"
}

variable "table_names" {
  description = "DynamoDB table names the CI role and runtime user may touch."
  type = object({
    karma      = string
    settings   = string
    workspaces = string
  })
  default = {
    karma      = "plusplus_karma"
    settings   = "plusplus_channel_settings"
    workspaces = "plusplus_workspaces"
  }
}

variable "create_github_oidc_provider" {
  description = "Create the account-level GitHub OIDC provider. Set false if one already exists in the account (it is a singleton per URL)."
  type        = bool
  default     = false
}

variable "create_runtime_access_key" {
  description = "If true, create an access key for the runtime user (stored in TF state). Prefer false and create the key manually."
  type        = bool
  default     = false
}

output "state_bucket_name" {
  description = "S3 bucket for Terraform remote state. Put this in terraform/backend.hcl."
  value       = aws_s3_bucket.state.bucket
}

output "ci_role_arn" {
  description = "IAM role ARN for GitHub Actions. Set as the AWS_ROLE_ARN repo variable."
  value       = aws_iam_role.ci.arn
}

output "runtime_user_name" {
  description = "IAM user for the Railway runtime. Create an access key for it."
  value       = aws_iam_user.runtime.name
}

output "runtime_access_key_id" {
  description = "Runtime access key id (only when create_runtime_access_key = true)."
  value       = try(aws_iam_access_key.runtime[0].id, null)
}

output "runtime_secret_access_key" {
  description = "Runtime secret (only when create_runtime_access_key = true)."
  value       = try(aws_iam_access_key.runtime[0].secret, null)
  sensitive   = true
}

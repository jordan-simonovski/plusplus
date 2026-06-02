output "karma_table_arn" {
  description = "ARN of the karma table."
  value       = aws_dynamodb_table.karma.arn
}

output "settings_table_arn" {
  description = "ARN of the channel settings table."
  value       = aws_dynamodb_table.channel_settings.arn
}

output "workspaces_table_arn" {
  description = "ARN of the workspaces table."
  value       = aws_dynamodb_table.workspaces.arn
}

output "metrics_dashboard_url" {
  description = "Console URL for the plusplus CloudWatch dashboard."
  value       = "https://${var.aws_region}.console.aws.amazon.com/cloudwatch/home?region=${var.aws_region}#dashboards/dashboard/${aws_cloudwatch_dashboard.metrics.dashboard_name}"
}

output "table_names" {
  description = "Map of logical name to DynamoDB table name."
  value = {
    karma      = aws_dynamodb_table.karma.name
    settings   = aws_dynamodb_table.channel_settings.name
    workspaces = aws_dynamodb_table.workspaces.name
  }
}

variable "aws_region" {
  description = "AWS region for the DynamoDB tables."
  type        = string
  default     = "ap-southeast-2"
}

variable "karma_table_name" {
  description = "Name of the karma totals table."
  type        = string
  default     = "plusplus_karma"
}

variable "settings_table_name" {
  description = "Name of the channel settings table."
  type        = string
  default     = "plusplus_channel_settings"
}

variable "workspaces_table_name" {
  description = "Name of the Slack workspaces table."
  type        = string
  default     = "plusplus_workspaces"
}

variable "selfhosted_table_names" {
  description = "DynamoDB table names for the self-hosted deployment. Must match terraform/bootstrap var.selfhosted_table_names."
  type = object({
    karma      = string
    settings   = string
    workspaces = string
  })
  default = {
    karma      = "plusplus_sh_karma"
    settings   = "plusplus_sh_channel_settings"
    workspaces = "plusplus_sh_workspaces"
  }
}

variable "point_in_time_recovery" {
  description = "Enable point-in-time recovery on the tables."
  type        = bool
  default     = true
}

variable "metrics_schedule" {
  description = "CloudWatch Events schedule for the metrics Lambda."
  type        = string
  default     = "rate(1 hour)"
}

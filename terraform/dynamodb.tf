# Note: provider 6.x deprecates hash_key/range_key in favor of key_schema, but
# key_schema inside global_secondary_index currently causes perpetual plan drift
# (hashicorp/terraform-provider-aws#46513). We keep the deprecated-but-correct
# form; it only emits a cosmetic warning. Do not "modernize" until that is fixed.
resource "aws_dynamodb_table" "karma" {
  name         = var.karma_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "team_id"
  range_key    = "user_id"

  attribute {
    name = "team_id"
    type = "S"
  }
  attribute {
    name = "user_id"
    type = "S"
  }
  attribute {
    name = "karma_total"
    type = "N"
  }

  global_secondary_index {
    name            = "leaderboard"
    hash_key        = "team_id"
    range_key       = "karma_total"
    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = var.point_in_time_recovery
  }
}

resource "aws_dynamodb_table" "channel_settings" {
  name         = var.settings_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "team_id"
  range_key    = "channel_id"

  attribute {
    name = "team_id"
    type = "S"
  }
  attribute {
    name = "channel_id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = var.point_in_time_recovery
  }
}

resource "aws_dynamodb_table" "workspaces" {
  name         = var.workspaces_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "team_id"

  attribute {
    name = "team_id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = var.point_in_time_recovery
  }
}

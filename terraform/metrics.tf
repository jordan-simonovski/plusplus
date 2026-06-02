# Hourly Lambda that counts items in the DynamoDB tables and publishes them as
# custom CloudWatch metrics under the "plusplus" namespace, rendered by the
# dashboard below. Serverless: no host to run, ~$0 at any sane scale.

data "archive_file" "metrics_lambda" {
  type        = "zip"
  source_file = "${path.module}/lambda/metrics.py"
  output_path = "${path.module}/build/metrics_lambda.zip"
}

resource "aws_iam_role" "metrics_lambda" {
  name = "plusplus-metrics-lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "metrics_lambda" {
  name = "plusplus-metrics-lambda"
  role = aws_iam_role.metrics_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "CountTables"
        Effect = "Allow"
        Action = ["dynamodb:Scan", "dynamodb:DescribeTable"]
        Resource = [
          aws_dynamodb_table.karma.arn,
          aws_dynamodb_table.channel_settings.arn,
          aws_dynamodb_table.workspaces.arn,
        ]
      },
      {
        Sid      = "PublishMetrics"
        Effect   = "Allow"
        Action   = "cloudwatch:PutMetricData"
        Resource = "*"
        Condition = {
          StringEquals = { "cloudwatch:namespace" = "plusplus" }
        }
      },
      {
        Sid    = "Logs"
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.metrics.arn}:*"
      },
    ]
  })
}

resource "aws_cloudwatch_log_group" "metrics" {
  name              = "/aws/lambda/plusplus-metrics"
  retention_in_days = 14
}

resource "aws_lambda_function" "metrics" {
  function_name    = "plusplus-metrics"
  role             = aws_iam_role.metrics_lambda.arn
  runtime          = "python3.13"
  handler          = "metrics.handler"
  filename         = data.archive_file.metrics_lambda.output_path
  source_code_hash = data.archive_file.metrics_lambda.output_base64sha256
  timeout          = 60

  environment {
    variables = {
      WORKSPACES_TABLE = aws_dynamodb_table.workspaces.name
      KARMA_TABLE      = aws_dynamodb_table.karma.name
      SETTINGS_TABLE   = aws_dynamodb_table.channel_settings.name
    }
  }

  depends_on = [aws_cloudwatch_log_group.metrics]
}

# Plain CloudWatch Events cron rule -> Lambda. Resource-based permission lets
# the rule invoke directly; no EventBridge Scheduler / invoke role needed.
resource "aws_cloudwatch_event_rule" "metrics" {
  name                = "plusplus-metrics"
  schedule_expression = var.metrics_schedule
}

resource "aws_cloudwatch_event_target" "metrics" {
  rule = aws_cloudwatch_event_rule.metrics.name
  arn  = aws_lambda_function.metrics.arn
}

resource "aws_lambda_permission" "metrics_events" {
  statement_id  = "AllowExecutionFromCloudWatchEvents"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.metrics.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.metrics.arn
}

resource "aws_cloudwatch_dashboard" "metrics" {
  dashboard_name = "plusplus"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "plusplus totals"
          region = var.aws_region
          view   = "timeSeries"
          stat   = "Maximum"
          period = 3600
          metrics = [
            ["plusplus", "Workspaces"],
            ["plusplus", "Users"],
            ["plusplus", "ChannelSettings"],
          ]
        }
      },
    ]
  })
}

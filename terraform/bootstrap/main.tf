data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
  partition  = data.aws_partition.current.partition

  table_arns = [
    for name in values(var.table_names) :
    "arn:${local.partition}:dynamodb:${var.aws_region}:${local.account_id}:table/${name}"
  ]

  karma_table_arn = "arn:${local.partition}:dynamodb:${var.aws_region}:${local.account_id}:table/${var.table_names.karma}"
}

# ---------------------------------------------------------------------------
# Terraform remote state bucket
# ---------------------------------------------------------------------------
resource "aws_s3_bucket" "state" {
  bucket = var.state_bucket_name
}

resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "state" {
  bucket                  = aws_s3_bucket.state.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# ---------------------------------------------------------------------------
# GitHub Actions OIDC provider + CI role (short-lived credentials, no secrets)
# ---------------------------------------------------------------------------
# The OIDC provider is account-global (one per URL). Create it in a fresh
# account, or reference the existing one with create_github_oidc_provider=false.
resource "aws_iam_openid_connect_provider" "github" {
  count           = var.create_github_oidc_provider ? 1 : 0
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

data "aws_iam_openid_connect_provider" "github" {
  count = var.create_github_oidc_provider ? 0 : 1
  url   = "https://token.actions.githubusercontent.com"
}

locals {
  github_oidc_arn = var.create_github_oidc_provider ? aws_iam_openid_connect_provider.github[0].arn : data.aws_iam_openid_connect_provider.github[0].arn
}

data "aws_iam_policy_document" "ci_assume_role" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    # Allow the main branch (apply) and pull requests (plan) of this repo only.
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        "repo:${var.github_repository}:ref:refs/heads/main",
        "repo:${var.github_repository}:pull_request",
      ]
    }
  }
}

resource "aws_iam_role" "ci" {
  name               = "plusplus-ci-terraform"
  assume_role_policy = data.aws_iam_policy_document.ci_assume_role.json
}

data "aws_iam_policy_document" "ci" {
  statement {
    sid       = "TerraformStateBucket"
    effect    = "Allow"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.state.arn]
  }

  statement {
    sid       = "TerraformStateObjects"
    effect    = "Allow"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.state.arn}/plusplus/*"]
  }

  statement {
    sid    = "ManageDynamoTables"
    effect = "Allow"
    actions = [
      "dynamodb:CreateTable",
      "dynamodb:DeleteTable",
      "dynamodb:UpdateTable",
      "dynamodb:DescribeTable",
      "dynamodb:DescribeContinuousBackups",
      "dynamodb:UpdateContinuousBackups",
      "dynamodb:DescribeTimeToLive",
      "dynamodb:UpdateTimeToLive",
      "dynamodb:ListTagsOfResource",
      "dynamodb:TagResource",
      "dynamodb:UntagResource",
    ]
    resources = concat(local.table_arns, ["${local.karma_table_arn}/index/*"])
  }
}

resource "aws_iam_role_policy" "ci" {
  name   = "plusplus-ci-terraform"
  role   = aws_iam_role.ci.id
  policy = data.aws_iam_policy_document.ci.json
}

# ---------------------------------------------------------------------------
# Runtime IAM user for Railway (least privilege: read + item ops only)
# ---------------------------------------------------------------------------
resource "aws_iam_user" "runtime" {
  name = "plusplus-runtime"
}

data "aws_iam_policy_document" "runtime" {
  statement {
    sid       = "DescribeTables"
    effect    = "Allow"
    actions   = ["dynamodb:DescribeTable"]
    resources = local.table_arns
  }

  statement {
    sid    = "ItemOps"
    effect = "Allow"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:Query",
    ]
    resources = concat(local.table_arns, ["${local.karma_table_arn}/index/*"])
  }
}

resource "aws_iam_user_policy" "runtime" {
  name   = "plusplus-runtime-dynamodb"
  user   = aws_iam_user.runtime.name
  policy = data.aws_iam_policy_document.runtime.json
}

resource "aws_iam_access_key" "runtime" {
  count = var.create_runtime_access_key ? 1 : 0
  user  = aws_iam_user.runtime.name
}

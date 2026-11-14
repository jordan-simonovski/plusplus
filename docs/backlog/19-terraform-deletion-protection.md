# 19. DynamoDB deletion protection

## Owner merges this

`terraform/` is applied to production automatically when a change merges to `main`
(`.github/workflows/terraform.yml`, `terraform apply -auto-approve`). An agent may prepare
this PR, but must not merge it, and must put the plan output from the PR's workflow run in
the description for the owner to read.

## Problem

`terraform/dynamodb.tf` defines six tables (karma, channel settings and workspaces, each
in a hosted and a self-hosted set). Point-in-time recovery is configurable, but none of
the tables sets `deletion_protection_enabled`. A plan that replaces a table (a renamed
resource, a changed key schema, a mistaken variable) would be applied without a prompt and
would delete every workspace's karma. Point-in-time recovery does not survive the table
being deleted in a way that makes this painless: it restores to a new table, by hand.

## Change

1. Add `deletion_protection_enabled = true` to all six `aws_dynamodb_table` resources.
2. Add `lifecycle { prevent_destroy = true }` to the three hosted tables, so Terraform
   refuses to plan their destruction at all.

## Done when

- `terraform fmt -check -recursive` passes, and `terraform validate` passes after
  `terraform init -backend=false` (the real backend needs AWS credentials you will not
  have). The plan from the PR's `terraform` workflow run is the one that counts.
- Not open at the same time as item 03, which edits the same file.
- The PR's plan shows six in-place updates and no replacements or destroys. Paste it.
- No changeset.

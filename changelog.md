# Changelog

## v1 (2026-02-24)

- Fixed provider schema alignment: `action.target` and `action.parameter` changed to `list(object({key, value}))` nested blocks; `target.resource_tag` changed to nested block.
- Fixed `selection_mode` validation to use `replace()`+`tonumber()` instead of brittle `regex(...)[0]` capture indexing.
- Fixed template ARN output: constructed via `arn:aws:fis:{region}:{account_id}:experiment-template/{id}` since provider only exports `id`.
- Fixed `log_group_arn`: removed double `:*` suffix since `aws_cloudwatch_log_group.arn` already includes it.
- Made `targets` optional in experiment template schema to support targetless actions like `aws:fis:wait`.
- Added `data "aws_region" "current" {}` for ARN construction.
- Added mutual exclusivity validation for `resource_arns` vs `resource_tags`.
- Added optional `experiment_options` block with `account_targeting` (default `"single-account"`) and `empty_target_resolution_mode` (default `"fail"`).
- Added optional `experiment_report_configuration` block with S3 output, CloudWatch dashboard data sources, and pre/post experiment duration.
- Added requirements 4.17 (experiment_options), 4.18 (experiment_report_configuration), and 6.4 (account_targeting pass-through).

## v0 (2026-02-24)

- Added initial requirements and design specs for the AWS FIS Terraform module.
- Defined provider-aligned experiment template modeling with dynamic action/target/stop-condition blocks.
- Standardized on a single shared CloudWatch log group for periodic experiments (`/aws/fis/experiments/{environment}`) with 30-day retention.
- Locked IAM scope: module resolves existing hardcoded `FISExperimentRole` and does not create IAM roles or policies.
- Added IAM prerequisite contract covering trust policy and required permission categories (FIS actions, S3, KMS, CloudWatch Logs).
- Scoped infrastructure to internal S3/KMS modules and existing target resources only (no workload resource creation).
- Added minimal module-level validation strategy, including `selection_mode` format and bounds checks.
- Documented Terratest + property-based testing strategy and cleanup expectations.

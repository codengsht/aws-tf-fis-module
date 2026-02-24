# Changelog

## v0 (2026-02-24)

- Added initial requirements and design specs for the AWS FIS Terraform module.
- Defined provider-aligned experiment template modeling with dynamic action/target/stop-condition blocks.
- Standardized on a single shared CloudWatch log group for periodic experiments (`/aws/fis/experiments/{environment}`) with 30-day retention.
- Locked IAM scope: module resolves existing hardcoded `FISExperimentRole` and does not create IAM roles or policies.
- Added IAM prerequisite contract covering trust policy and required permission categories (FIS actions, S3, KMS, CloudWatch Logs).
- Scoped infrastructure to internal S3/KMS modules and existing target resources only (no workload resource creation).
- Added minimal module-level validation strategy, including `selection_mode` format and bounds checks.
- Documented Terratest + property-based testing strategy and cleanup expectations.

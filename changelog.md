# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2026-02-24

### Added

- FIS experiment template creation via provider-aligned `experiment_templates` input schema
- S3 bucket provisioning for Lambda fault-injection configuration artifacts (via internal S3_Module)
- KMS key provisioning for S3 bucket encryption (via internal KMS_Module)
- Shared CloudWatch Logs log group (`/aws/fis/experiments/{environment}`) with 30-day retention
- IAM role lookup for pre-existing `FISExperimentRole`
- Support for experiment options (`account_targeting`, `empty_target_resolution_mode`)
- Support for experiment report configuration (S3 output, CloudWatch dashboard data sources)
- Selection mode validation (`ALL`, `COUNT(n)`, `PERCENT(n)`)
- S3-safe `ci_commit_ref_name` validation
- Mutual exclusivity validation for `resource_arns` / `resource_tags`
- Property-based tests (12 properties) using Go rapid library
- Terratest integration test for full module deployment validation

### Deferred

- Mandatory target tag-gating enforcement (optional tag selectors accepted but not enforced)
- Multi-account experiment support (scoped to single-account only)

# Changelog

## v1 (2026-02-24)
- Expanded FIS template requirements to include optional `experiment_options` and optional `experiment_report_configuration` passthrough blocks.
- Clarified single-account behavior by defaulting `experiment_options.account_targeting` to `single-account` when provided.
- Extended design to align template schema with provider-style nested blocks (`action.target`, `action.parameter`, `target.resource_tag`) and optional `targets` support for targetless actions.
- Added design coverage for experiment report configuration (S3 outputs, dashboard data sources, pre/post experiment durations).
- Added `aws_region` data-source usage in design for constructing experiment template ARNs from exported template IDs.
- Added Kiro sync hook files for requirements/design/tasks change workflows under `.kiro/hooks/`.
- Added empty `.kiro/specs/aws-fis-terraform-module/tasks.md` scaffold file.
- Updated VS Code setting `kiroAgent.configureMCP` from `Disabled` to `Enabled`.

## v0 (2026-02-24)
- Added initial requirements and design specs for the AWS FIS Terraform module
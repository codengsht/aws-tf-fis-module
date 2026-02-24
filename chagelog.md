# Changelog

## v2 (2026-02-24)
- Strengthened S3 bucket naming requirements by adding explicit `ci_commit_ref_name` constraints: lowercase alphanumeric plus hyphen only, no leading/trailing hyphen, and no consecutive hyphens.
- Added target-selection quality requirement for `resource_tags`: when tags are used, at least one entry is required and each tag must have non-empty trimmed `key` and `value`.
- Updated design with caller-side GitLab ref sanitization guidance for `CI_COMMIT_REF_NAME`.
- Added design-level validation and error handling for invalid `ci_commit_ref_name` formats and blank `resource_tags` entries.
- Expanded correctness properties and tests (including new property coverage) for commit-ref and target-tag validation behavior.

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

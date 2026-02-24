# Implementation Plan: AWS FIS Terraform Module

## Overview

Incrementally build the FIS Terraform module file-by-file, starting with foundational inputs and data sources, then layering in KMS, S3, CloudWatch, experiment templates, outputs, and finally validation logic. Property-based tests (Go/rapid) and Terratest integration tests are woven in close to the code they verify.

## Tasks

- [ ] 1. Create foundational module files (variables.tf, main.tf)
  - [ ] 1.1 Create `variables.tf` with required input variables
    - Define `environment` (string, required, non-empty validation)
    - Define `ci_commit_ref_name` (string, required, with validation blocks: non-empty, S3-safe characters regex `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, no consecutive hyphens)
    - Define `experiment_templates` variable with the full `map(object({...}))` type schema from the design (actions, targets, stop_conditions, tags, experiment_options, experiment_report_configuration)
    - Include validation blocks on `experiment_templates`: mutual exclusivity of `resource_arns`/`resource_tags`, non-empty `resource_tags` key/value
    - _Requirements: 2.4, 2.7, 4.1, 4.5, 4.6, 4.7, 4.8, 4.15, 4.17, 4.18, 7.7_

  - [ ] 1.2 Create `main.tf` with provider configuration and data sources
    - Add `data.aws_caller_identity.current` for account ID resolution
    - Add `data.aws_region.current` for region name (used in ARN construction)
    - Add `data.aws_iam_role.fis_experiment_role` with hardcoded name `FISExperimentRole`
    - _Requirements: 2.3, 3.1, 3.2, 3.3, 3.5, 3.6_


- [ ] 2. Implement KMS and S3 modules (kms.tf, s3.tf) with validation
  - [ ] 2.1 Create `kms.tf` with KMS_Module reference from Artifactory
    - Invoke internal KMS_Module for S3 encryption key
    - Pass environment tag
    - _Requirements: 1.2, 1.4, 2.2_

  - [ ] 2.2 Create `s3.tf` with S3_Module reference from Artifactory
    - Invoke internal S3_Module with bucket name `fis-lambda-config-${data.aws_caller_identity.current.account_id}-${var.ci_commit_ref_name}`
    - Pass `kms_key_arn` from `module.fis_kms`
    - Add bucket name length validation (≤63 characters) via `locals` + `precondition` or `check` block
    - _Requirements: 1.1, 1.4, 2.1, 2.2, 2.5, 2.6_

  - [ ]* 2.3 Write property test for S3 bucket name construction (Property 1)
    - **Property 1: S3 Bucket Name Construction**
    - Generate random 12-digit `account_id` and valid `ci_commit_ref_name` strings; verify bucket name equals `"fis-lambda-config-${account_id}-${ci_commit_ref_name}"`
    - Pure function test — no infrastructure needed
    - **Validates: Requirements 2.5**

  - [ ]* 2.4 Write property test for S3 bucket name length validation (Property 2)
    - **Property 2: S3 Bucket Name Length Validation**
    - Generate `ci_commit_ref_name` of varying lengths; verify validation accepts names ≤63 chars and rejects names >63 chars
    - Pure function test — no infrastructure needed
    - **Validates: Requirements 2.6**

  - [ ]* 2.5 Write property test for ci_commit_ref_name S3-safe character validation (Property 12)
    - **Property 12: ci_commit_ref_name S3-Safe Character Validation**
    - Generate strings with valid chars (`[a-z0-9-]`), invalid chars (uppercase, underscores, slashes, periods), leading/trailing hyphens, and consecutive hyphens; verify validation accepts only S3-safe values
    - Pure function test — no infrastructure needed
    - **Validates: Requirements 2.7**

- [ ] 3. Checkpoint - Ensure foundational module and S3/KMS compile
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 4. Implement CloudWatch logs and experiment templates
  - [ ] 4.1 Create `logs.tf` with shared CloudWatch log group
    - Create `aws_cloudwatch_log_group.fis_experiments` with name `/aws/fis/experiments/${var.environment}`
    - Set `retention_in_days = 30`
    - Add environment tag
    - _Requirements: 5.1, 5.2, 5.6, 5.7_

  - [ ] 4.2 Create `fis_templates.tf` with experiment template resources
    - Implement `aws_fis_experiment_template.this` with `for_each = var.experiment_templates`
    - Wire `role_arn` from `data.aws_iam_role.fis_experiment_role.arn`
    - Implement dynamic blocks: `action` (with nested `target`, `parameter`), `target` (with nested `resource_tag`, `filter`), `stop_condition`, `experiment_options`, `experiment_report_configuration` (with nested `outputs`/`s3_configuration`, `data_sources`/`cloudwatch_dashboards`)
    - Implement `log_configuration` block referencing `aws_cloudwatch_log_group.fis_experiments.arn` with `log_schema_version = 2`
    - Set Name tag to `fis-${each.key}-${var.environment}`
    - _Requirements: 4.1, 4.2, 4.3, 4.5, 4.6, 4.7, 4.8, 4.16, 4.17, 4.18, 5.3, 6.4_

  - [ ] 4.3 Add selection_mode validation logic in `fis_templates.tf` or `variables.tf`
    - Implement `locals` block parsing selection_mode into components (is_all, is_count, is_percent, numeric_value)
    - Add preconditions for: invalid format, COUNT(n) where n ≤ 0, PERCENT(n) where n < 1 or n > 100
    - _Requirements: 4.9, 4.10, 4.11, 4.12_

  - [ ]* 4.4 Write property test for selection mode validation (Property 7)
    - **Property 7: Selection Mode Validation**
    - Generate valid and invalid selection_mode strings (ALL, COUNT(n), PERCENT(n) with in-bounds and out-of-bounds values, malformed formats); verify validation accepts/rejects correctly
    - Pure function test — no infrastructure needed
    - **Validates: Requirements 4.10, 4.11, 4.12**

  - [ ]* 4.5 Write property test for stop condition default (Property 5)
    - **Property 5: Stop Condition Default**
    - Generate templates with and without stop_conditions; verify that when not provided, the default is a single stop condition with `source = "none"`
    - **Validates: Requirements 4.5**

  - [ ]* 4.6 Write property test for selection mode default (Property 6)
    - **Property 6: Selection Mode Default**
    - Generate targets with and without selection_mode; verify default is `"ALL"`
    - **Validates: Requirements 4.7**

  - [ ]* 4.7 Write property test for template naming convention (Property 8)
    - **Property 8: Template Naming Convention**
    - Generate template map keys (`{service}-{scenario}`) and environment values; verify Name tag equals `"fis-{service}-{scenario}-{environment}"`
    - Pure function test — no infrastructure needed
    - **Validates: Requirements 4.16**

  - [ ]* 4.8 Write property test for single deterministic log group (Property 9)
    - **Property 9: Single Deterministic Log Group**
    - Generate varying numbers of templates with a fixed environment; verify exactly one log group named `"/aws/fis/experiments/${environment}"`
    - **Validates: Requirements 5.1, 5.2**

- [ ] 5. Checkpoint - Ensure templates and logs compile
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 6. Implement outputs and remaining validation
  - [ ] 6.1 Create `outputs.tf` with all module outputs
    - Output `experiment_role_arn` from `data.aws_iam_role.fis_experiment_role.arn`
    - Output `s3_bucket_name` and `s3_bucket_arn` from `module.fis_s3`
    - Output `kms_key_id` and `kms_key_arn` from `module.fis_kms`
    - Output `experiment_templates` map with `id`, `arn` (constructed via `locals`), and `name` per template
    - Add `locals` block for constructed ARNs: `arn:aws:fis:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:experiment-template/${id}`
    - Output `log_group_name` and `log_group_arn` from `aws_cloudwatch_log_group.fis_experiments`
    - _Requirements: 3.4, 9.1, 9.2, 9.3, 9.4, 9.5_

  - [ ]* 6.2 Write property test for template count equals input count (Property 3)
    - **Property 3: Template Count Equals Input Count**
    - Generate experiment_templates maps of varying sizes (1-10); verify output count matches input count
    - **Validates: Requirements 4.2, 9.4**

  - [ ]* 6.3 Write property test for uniform template configuration (Property 4)
    - **Property 4: Uniform Template Configuration**
    - Generate multiple templates; verify all share the same `role_arn` and `log_group_arn`
    - **Validates: Requirements 4.3, 5.3**

  - [ ]* 6.4 Write property test for non-empty target identifier validation (Property 10)
    - **Property 10: Non-Empty Target Identifier Validation**
    - Generate targets with empty/non-empty `resource_arns` and `resource_tags` (including blank key/value entries); verify validation accepts/rejects correctly
    - **Validates: Requirements 7.2, 7.7**

  - [ ]* 6.5 Write property test for output map completeness (Property 11)
    - **Property 11: Output Map Completeness**
    - Generate template keys; verify output map has matching entries with non-empty `id`, `arn`, `name`; verify ARN follows `arn:aws:fis:{region}:{account_id}:experiment-template/{id}` pattern
    - **Validates: Requirements 9.4**

- [ ] 7. Checkpoint - Ensure full module compiles with all outputs and validation
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 8. Implement Terratest integration tests
  - [ ] 8.1 Create test fixtures directory (`tests/fixtures/`)
    - Create `tests/fixtures/main.tf` — invoke internal modules to provision one target resource per service (S3, Kinesis, DynamoDB, Lambda, Network), then invoke the FIS_Module with provisioned targets
    - Create `tests/fixtures/variables.tf` — test input variables
    - Create `tests/fixtures/outputs.tf` — expose FIS_Module outputs for assertions
    - Create `tests/fixtures/terraform.tfvars` — default test values
    - _Requirements: 10.1, 10.2_

  - [ ]* 8.2 Write Terratest integration test (`tests/fis_module_test.go`)
    - Implement `TestFISModuleFullDeployment` using Terratest
    - Use `defer terraform.Destroy(t, terraformOptions)` for cleanup
    - Assert experiment templates created for each service target
    - Assert S3 bucket name and ARN outputs are non-empty and correct
    - Assert KMS key ID and ARN outputs are non-empty
    - Assert shared CloudWatch log group name and ARN outputs are non-empty and correct
    - Assert each experiment template references the resolved `FISExperimentRole` ARN
    - Assert experiment template IDs, ARNs, and names are present in outputs
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9_

- [ ] 9. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests for Properties 1, 2, 7, 8, 12 are pure function tests (no infrastructure); Properties 3, 4, 5, 6, 9, 10, 11 require Terraform plan/apply and are best validated via Terratest with property-based input generation
- All 12 correctness properties from the design document are covered by property test tasks
- The module uses HCL (Terraform) for infrastructure code and Go for all tests (rapid + Terratest)

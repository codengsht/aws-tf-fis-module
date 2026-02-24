# Requirements Document

## Introduction

This document defines requirements for an AWS Fault Injection Service (FIS) Terraform module that enables controlled chaos engineering experiments. The module provisions supporting infrastructure (one S3 bucket encrypted with KMS for Lambda fault-injection configuration artifacts), creates `aws_fis_experiment_template` resources from structured input, and references existing IAM roles.

The module does NOT create IAM roles, IAM policies, or workload resources (Kinesis streams, DynamoDB tables, Lambda functions, VPC/subnet/network resources). Those resources are managed externally and passed in as module inputs.

The module uses internal Terraform modules from Artifactory for S3 and KMS. Tag-based enforcement for target resources is intentionally deferred in this release.

The module input model for experiment templates is provider-aligned: it mirrors Terraform AWS provider `aws_fis_experiment_template` concepts where possible. Service-specific and cross-field experiment validity is primarily enforced by Terraform provider and AWS FIS API validation, while this module applies only minimal module-level validation.

## Glossary

- **FIS_Module**: Terraform module that provisions FIS supporting infrastructure and experiment templates
- **FISExperimentRole**: Default IAM role name used by FIS for experiment execution
- **Experiment_Role_Name**: Hardcoded IAM role name `FISExperimentRole` used for IAM lookup
- **Experiment_Role_Arn**: ARN resolved from IAM role lookup and attached to experiment templates
- **S3_Module**: Existing internal Terraform module for S3 bucket provisioning (Artifactory)
- **KMS_Module**: Existing internal Terraform module for KMS key provisioning (Artifactory)
- **Experiment_Template**: Terraform `aws_fis_experiment_template` resource
- **Target_Resource**: Existing AWS resource selected as a fault injection target
- **Selection_Mode**: FIS target selection mode (`ALL`, `COUNT(n)`, `PERCENT(n)`)
- **Action_Dependency**: FIS action ordering via `start_after`
- **Log_Configuration**: FIS experiment template configuration for CloudWatch logs
- **Environment**: Required module input used in naming conventions (`fis-{service}-{scenario}-{environment}` and shared log group naming)
- **ci_commit_ref_name**: Required module input variable sourced from the GitLab CI/CD `CI_COMMIT_REF_NAME` environment variable; used in resource naming (for example, S3 bucket name)
- **Terratest**: Go-based integration testing framework used to validate Terraform module behavior by provisioning real infrastructure and running assertions

## Requirements

### Requirement 1: Module Structure and Internal Module References

**User Story:** As a platform engineer, I want this module to reuse internal infrastructure modules, so that implementation follows organizational standards.

#### Acceptance Criteria

1. THE FIS_Module SHALL reference the S3_Module from Artifactory for S3 bucket provisioning.
2. THE FIS_Module SHALL reference the KMS_Module from Artifactory for KMS key provisioning required by the S3 bucket.
3. THE FIS_Module SHALL organize resources into service-oriented Terraform files (for example: `s3.tf`, `kms.tf`, `fis_templates.tf`, `logs.tf`, `variables.tf`, `outputs.tf`, `main.tf`).
4. THE FIS_Module SHALL NOT provision S3 or KMS resources directly when equivalent internal modules exist.
5. THE FIS_Module SHALL NOT invoke a Lambda provisioning module as part of this feature.

### Requirement 2: S3 Bucket for Lambda Configuration Storage

**User Story:** As a platform engineer, I want a single encrypted S3 bucket for Lambda fault injection configuration data, so that experiments use centralized and compliant storage.

#### Acceptance Criteria

1. THE FIS_Module SHALL provision exactly one S3 bucket using the S3_Module.
2. THE FIS_Module SHALL provision a KMS key using the KMS_Module and configure S3 encryption with that key.
3. THE FIS_Module SHALL resolve the AWS account ID automatically using `data.aws_caller_identity` and SHALL NOT require `account_id` as a module input.
4. THE FIS_Module SHALL accept `ci_commit_ref_name` as a required input variable (sourced from the GitLab CI/CD `CI_COMMIT_REF_NAME` environment variable).
5. THE FIS_Module SHALL construct the bucket name from `fis-lambda-config-${account_id}-${ci_commit_ref_name}`.
6. THE FIS_Module SHALL validate that the final bucket name satisfies S3 naming constraints, including maximum length of 63 characters.
7. THE FIS_Module SHALL validate that `ci_commit_ref_name` contains only lowercase letters, numbers, and hyphens (`[a-z0-9-]`), does not start or end with a hyphen, and does not contain consecutive hyphens, to ensure the resulting S3 bucket name is fully S3-compliant and DNS-compatible.

### Requirement 3: Experiment Role Reference and Validation

**User Story:** As a platform engineer, I want the module to resolve the experiment execution role from existing IAM, so FIS templates always use a valid pre-provisioned role.

#### Acceptance Criteria

1. THE FIS_Module SHALL hardcode the IAM role name as `FISExperimentRole`.
2. THE FIS_Module SHALL resolve the role using IAM lookup by the hardcoded name `FISExperimentRole`.
3. IF the role does not exist, THEN THE FIS_Module SHALL fail with a clear validation/error message.
4. THE FIS_Module SHALL output the resolved Experiment_Role_Arn.
5. THE FIS_Module SHALL NOT create IAM roles.
6. THE FIS_Module SHALL NOT create IAM policies.

### Requirement 4: Experiment Template Schema and Creation

**User Story:** As a platform engineer, I want to define FIS templates through a provider-aligned input schema, so templates are consistent while keeping module logic maintainable.

#### Acceptance Criteria

1. THE FIS_Module SHALL accept a structured `experiment_templates` input (map/object schema) that mirrors `aws_fis_experiment_template` arguments and blocks as closely as practical.
2. THE FIS_Module SHALL create one Experiment_Template per `experiment_templates` entry.
3. THE FIS_Module SHALL assign the resolved Experiment_Role_Arn (from the hardcoded `FISExperimentRole` lookup) to every Experiment_Template.
4. THE FIS_Module SHALL support template targets for this initial service scope only: S3, Kinesis, DynamoDB, Lambda, and Network.
5. THE FIS_Module SHALL require at least one stop condition per template; WHEN not provided, THE FIS_Module SHALL default to a stop condition with `source = "none"`.
6. THE FIS_Module SHALL accept optional target filters and optional target tags for target resolution.
7. WHEN Selection_Mode is not specified for a target, THE FIS_Module SHALL default it to `ALL`.
8. THE FIS_Module SHALL accept optional `start_after` action dependencies and optional action parameters, including duration values passed through as provided.
9. THE FIS_Module SHALL implement minimal module-level validation only (for example: non-empty required module inputs, non-empty target identifier values when directly provided, and Selection_Mode bounds checks).
10. WHEN Selection_Mode is `COUNT(n)`, THE FIS_Module SHALL validate that `n` is an integer greater than 0.
11. WHEN Selection_Mode is `PERCENT(n)`, THE FIS_Module SHALL validate that `n` is an integer from 1 through 100.
12. IF Selection_Mode format is invalid or `n` is out of bounds, THEN THE FIS_Module SHALL fail with a descriptive validation error.
13. THE FIS_Module SHALL NOT implement exhaustive per-service cross-field pre-validation for all experiment cases.
14. THE Terraform AWS provider and AWS FIS API SHALL be the authoritative validators for action/target compatibility and detailed parameter correctness.
15. THE FIS_Module SHALL accept a required `environment` input variable and use it in naming.
16. THE FIS_Module SHALL name templates using the convention `fis-{service}-{scenario}-{environment}`.
17. THE FIS_Module SHALL accept an optional `experiment_options` block per template with `account_targeting` (default `"single-account"`) and `empty_target_resolution_mode` (default `"fail"`).
18. THE FIS_Module SHALL accept an optional `experiment_report_configuration` block per template with S3 output configuration, CloudWatch dashboard data sources, and pre/post experiment duration settings.

### Requirement 5: CloudWatch Logs Configuration

**User Story:** As a module user, I want deterministic experiment logs, so execution evidence is easy to discover and audit.

#### Acceptance Criteria

1. THE FIS_Module SHALL create a single shared CloudWatch Logs log group for all Experiment_Templates managed by this module.
2. THE FIS_Module SHALL use the default naming pattern `/aws/fis/experiments/{environment}` for the shared log group.
3. THE FIS_Module SHALL attach the shared log group to each template Log_Configuration.
4. THE FIS_Module SHALL output the shared log group name and ARN.
5. THE FIS_Module SHALL document that a shared log group is the default for periodic experiment execution and low FIS log volume.
6. THE FIS_Module SHALL require the `environment` input used in shared log group naming.
7. THE FIS_Module SHALL configure the shared CloudWatch Logs log group with a retention period of 30 days.

### Requirement 6: No Multi-Account Experiment Support

**User Story:** As a security engineer, I want multi-account experiments excluded, so experiments cannot be configured to impact external AWS accounts.

#### Acceptance Criteria

1. THE FIS_Module SHALL NOT include configuration for multi-account FIS experiments.
2. THE FIS_Module SHALL document that multi-account experiments are out of scope.
3. THE FIS_Module SHALL scope template target references to the current account context where applicable.
4. WHEN `experiment_options.account_targeting` is provided, THE FIS_Module SHALL pass the value through to the provider; the default value `"single-account"` reinforces single-account scope.

### Requirement 7: Target Resource Inputs and Non-Creation Boundaries

**User Story:** As a module user, I want to pass existing target resources, so the module can orchestrate experiments without creating application infrastructure.

#### Acceptance Criteria

1. THE FIS_Module SHALL accept target resource identifiers (name, ARN, or tag selectors) for S3, Kinesis, DynamoDB, Lambda, and supported network targets.
2. THE FIS_Module SHALL validate that required target identifier inputs are non-empty.
3. THE FIS_Module SHALL NOT create Kinesis resources.
4. THE FIS_Module SHALL NOT create DynamoDB resources.
5. THE FIS_Module SHALL NOT create Lambda functions.
6. THE FIS_Module SHALL NOT create network infrastructure resources.
7. WHEN a target uses `resource_tags` for selection, THE FIS_Module SHALL validate that at least one tag entry is provided and that each tag entry has a non-empty (after trimming) `key` and a non-empty (after trimming) `value`.

### Requirement 8: Tag-Gating Roadmap (Deferred)

**User Story:** As a security engineer, I want clear scope on tag-gating controls, so current behavior is explicit and future hardening is planned.

#### Acceptance Criteria

1. THE FIS_Module SHALL document that mandatory target tag-gating enforcement is deferred in this release.
2. THE FIS_Module SHALL allow optional target tag selectors in template inputs.
3. THE FIS_Module SHALL NOT require a fixed opt-in tag (for example `managedByFIS`) in this release.

### Requirement 9: Module Outputs

**User Story:** As a module user, I want complete outputs for all managed artifacts, so downstream integration is reliable.

#### Acceptance Criteria

1. THE FIS_Module SHALL output the resolved Experiment_Role_Arn.
2. THE FIS_Module SHALL output S3 bucket name and ARN.
3. THE FIS_Module SHALL output KMS key ID and ARN.
4. THE FIS_Module SHALL output Experiment_Template IDs, ARNs, and names.
5. THE FIS_Module SHALL output the single shared CloudWatch log group name and ARN.
6. THE FIS_Module SHALL keep output behavior consistent with minimal module-level validation and provider/API authoritative validation.

### Requirement 10: Terratest Integration Testing

**User Story:** As a platform engineer, I want automated integration tests using Terratest, so that the FIS module is validated end-to-end against real AWS infrastructure.

#### Acceptance Criteria

1. THE Terratest SHALL provision one target resource per supported service (S3, Kinesis, DynamoDB, Lambda, Network) using internal modules (S3_Module, KMS_Module, Lambda_Module, and equivalent modules for Kinesis, DynamoDB, and Network).
2. THE Terratest SHALL invoke the FIS_Module with the provisioned target resources as inputs.
3. THE Terratest SHALL validate that Experiment_Templates are created for each supported service target.
4. THE Terratest SHALL validate that the FIS_Module outputs include the S3 bucket name and ARN.
5. THE Terratest SHALL validate that the FIS_Module outputs include the KMS key ID and ARN.
6. THE Terratest SHALL validate that the FIS_Module outputs include the single shared CloudWatch log group name and ARN.
7. THE Terratest SHALL validate that each Experiment_Template references the resolved Experiment_Role_Arn.
8. THE Terratest SHALL validate that the FIS_Module outputs include Experiment_Template IDs, ARNs, and names.
9. THE Terratest SHALL perform resource cleanup and teardown (for example, `defer terraform.Destroy()`) after test execution to ensure all provisioned resources are destroyed.

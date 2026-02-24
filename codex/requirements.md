# Requirements Document

## Introduction

This document defines requirements for an AWS Fault Injection Service (FIS) Terraform module that enables controlled chaos engineering experiments. The module provisions supporting infrastructure (one S3 bucket encrypted with KMS for Lambda fault-injection configuration artifacts), creates `aws_fis_experiment_template` resources from structured input, and references existing IAM roles.

The module does NOT create IAM roles, IAM policies, or workload resources (Kinesis streams, DynamoDB tables, Lambda functions, VPC/subnet/network resources). Those resources are managed externally and passed in as module inputs.

The module uses internal Terraform modules from Artifactory for S3 and KMS. Tag-based enforcement for target resources is intentionally deferred in this release.

## Glossary

- **FIS_Module**: Terraform module that provisions FIS supporting infrastructure and experiment templates
- **FISExperimentRole**: Default IAM role name used by FIS for experiment execution
- **Experiment_Role_Name**: Input role name used for IAM lookup; defaults to `FISExperimentRole`
- **Experiment_Role_Arn**: ARN resolved from IAM role lookup and attached to experiment templates
- **S3_Module**: Existing internal Terraform module for S3 bucket provisioning (Artifactory)
- **KMS_Module**: Existing internal Terraform module for KMS key provisioning (Artifactory)
- **Experiment_Template**: Terraform `aws_fis_experiment_template` resource
- **Target_Resource**: Existing AWS resource selected as a fault injection target
- **Selection_Mode**: FIS target selection mode (`ALL`, `COUNT(n)`, `PERCENT(n)`)
- **Action_Dependency**: FIS action ordering via `start_after`
- **Log_Configuration**: FIS experiment template configuration for CloudWatch logs

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
3. THE FIS_Module SHALL construct the bucket name from `fis-lambda-config-${account_id}-${ci_commit_ref_name}`.
4. THE FIS_Module SHALL validate that the final bucket name satisfies S3 naming constraints, including maximum length of 63 characters.

### Requirement 3: Experiment Role Reference and Validation

**User Story:** As a platform engineer, I want the module to resolve the experiment execution role from existing IAM, so FIS templates always use a valid pre-provisioned role.

#### Acceptance Criteria

1. THE FIS_Module SHALL accept an input variable `experiment_role_name` with default value `FISExperimentRole`.
2. THE FIS_Module SHALL resolve the role using IAM lookup by role name.
3. IF the role does not exist, THEN THE FIS_Module SHALL fail with a clear validation/error message.
4. THE FIS_Module SHALL output the resolved Experiment_Role_Arn.
5. THE FIS_Module SHALL NOT create IAM roles.
6. THE FIS_Module SHALL NOT create IAM policies.

### Requirement 4: Experiment Template Schema and Creation

**User Story:** As a platform engineer, I want to define FIS templates through a strict input schema, so templates are consistent and validation is deterministic.

#### Acceptance Criteria

1. THE FIS_Module SHALL accept a structured `experiment_templates` input (map/object schema) that defines template metadata, targets, actions, stop conditions, and tags.
2. THE FIS_Module SHALL create one Experiment_Template per `experiment_templates` entry.
3. THE FIS_Module SHALL assign the resolved Experiment_Role_Arn to every Experiment_Template.
4. THE FIS_Module SHALL support template targets for this initial service scope only: S3, Kinesis, DynamoDB, Lambda, and Network.
5. THE FIS_Module SHALL require at least one stop condition per template; WHEN not provided, THE FIS_Module SHALL default to a stop condition with `source = "none"`.
6. THE FIS_Module SHALL validate Selection_Mode values as `ALL`, `COUNT(n)`, or `PERCENT(n)`.
7. WHEN Selection_Mode is `COUNT(n)`, THE FIS_Module SHALL validate `n` is an integer greater than 0.
8. WHEN Selection_Mode is `PERCENT(n)`, THE FIS_Module SHALL validate `n` is an integer from 1 through 100.
9. THE FIS_Module SHALL fail validation for invalid Selection_Mode format or bounds.
10. WHEN Selection_Mode is not specified, THE FIS_Module SHALL default to `ALL`.
11. THE FIS_Module SHALL accept optional target filters and optional target tags for target resolution.
12. WHEN `start_after` is used, THE FIS_Module SHALL validate that referenced actions exist in the same template.
13. WHEN an action has a duration parameter, THE FIS_Module SHALL accept ISO 8601 duration format.
14. THE FIS_Module SHALL fail-fast when required target inputs for a template are missing.
15. THE FIS_Module SHALL name templates using the convention `fis-{service}-{scenario}-{environment}`.

### Requirement 5: CloudWatch Logs Configuration

**User Story:** As a module user, I want deterministic experiment logs, so execution evidence is easy to discover and audit.

#### Acceptance Criteria

1. THE FIS_Module SHALL create a single shared CloudWatch Logs log group for all Experiment_Templates managed by this module.
2. THE FIS_Module SHALL use the default naming pattern `/aws/fis/experiments/{environment}` for the shared log group.
3. THE FIS_Module SHALL attach the shared log group to each template Log_Configuration.
4. THE FIS_Module SHALL output the shared log group name and ARN.
5. THE FIS_Module SHALL document that a shared log group is the default for periodic experiment execution and low FIS log volume.

### Requirement 6: No Multi-Account Experiment Support

**User Story:** As a security engineer, I want multi-account experiments excluded, so experiments cannot be configured to impact external AWS accounts.

#### Acceptance Criteria

1. THE FIS_Module SHALL NOT include configuration for multi-account FIS experiments.
2. THE FIS_Module SHALL document that multi-account experiments are out of scope.
3. THE FIS_Module SHALL scope template target references to the current account context where applicable.

### Requirement 7: Target Resource Inputs and Non-Creation Boundaries

**User Story:** As a module user, I want to pass existing target resources, so the module can orchestrate experiments without creating application infrastructure.

#### Acceptance Criteria

1. THE FIS_Module SHALL accept target resource identifiers (name, ARN, or tag selectors) for S3, Kinesis, DynamoDB, Lambda, and supported network targets.
2. THE FIS_Module SHALL validate that required target identifier inputs are non-empty.
3. THE FIS_Module SHALL NOT create Kinesis resources.
4. THE FIS_Module SHALL NOT create DynamoDB resources.
5. THE FIS_Module SHALL NOT create Lambda functions.
6. THE FIS_Module SHALL NOT create network infrastructure resources.

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
5. THE FIS_Module SHALL output CloudWatch log group names and ARNs per template.
6. THE FIS_Module SHALL keep output behavior consistent with fail-fast validation for required inputs.

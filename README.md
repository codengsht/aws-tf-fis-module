# AWS FIS Terraform Module

Terraform module for AWS Fault Injection Service (FIS) that creates experiment templates, provisions a KMS-encrypted S3 bucket (via internal modules), creates a shared CloudWatch log group for experiment logs, and looks up the pre-existing `FISExperimentRole` IAM role.

## Usage

```hcl
module "fis" {
  source = "path/to/fis-module"

  environment        = "staging"
  ci_commit_ref_name = "main"

  experiment_templates = {
    "s3-pause-replication" = {
      description = "Pause S3 bucket replication"

      actions = {
        "pause-replication" = {
          action_id   = "aws:s3:bucket-pause-replication"
          description = "Pause replication on target bucket"
          targets = [
            { key = "Buckets", value = "target-buckets" }
          ]
          parameters = [
            { key = "duration", value = "PT5M" }
          ]
        }
      }

      targets = {
        "target-buckets" = {
          resource_type = "aws:s3:bucket"
          resource_arns = ["arn:aws:s3:::my-bucket"]
        }
      }
    }
  }
}
```

## Supported Service Scope

This module supports FIS experiment targets for the following AWS services:

- **S3** — Bucket-level fault injection (e.g., pause replication)
- **Kinesis** — Stream-level fault injection
- **DynamoDB** — Table-level fault injection
- **Lambda** — Function-level fault injection
- **Network** — Network-level fault injection (e.g., disrupt connectivity)

Service scope enforcement is handled by the AWS FIS provider and API, not by module-level validation. The module passes target `resource_type` and `action_id` values through to the provider, which validates them against the FIS service catalog.

## IAM Prerequisite Contract

The `FISExperimentRole` IAM role **must exist before** using this module. The module resolves the role via `data.aws_iam_role` lookup — it never creates IAM resources.

### Required Role Configuration

| Aspect | Requirement |
|---|---|
| Role Name | `FISExperimentRole` (hardcoded) |
| Trust Policy | Must trust `fis.amazonaws.com` as a service principal |
| FIS Permissions | Must allow FIS actions on S3, Kinesis, DynamoDB, Lambda, and Network targets |
| S3 Access | Must allow access to the `fis-lambda-config-*` bucket provisioned by this module |
| KMS Access | Must allow use of the KMS key provisioned by this module |
| CloudWatch Logs | Must allow writing to `/aws/fis/experiments/{environment}` |

If the role does not exist, `terraform plan` / `terraform apply` will fail with a clear error from the AWS API.

## Non-Creation Boundaries

This module is scoped to FIS experiment orchestration. It does **not** create workload or application infrastructure:

- Does **not** create Kinesis streams or related resources
- Does **not** create DynamoDB tables or related resources
- Does **not** create Lambda functions or invoke a Lambda provisioning module
- Does **not** create network infrastructure resources (VPCs, subnets, security groups, etc.)

Target resources must be provisioned externally and referenced via `resource_arns` or `resource_tags` in the `experiment_templates` input.

## Shared Log Group Rationale

The module creates a single shared CloudWatch Logs log group (`/aws/fis/experiments/{environment}`) for all experiment templates. This design is intentional:

- FIS experiments are typically run periodically (not continuously), producing low log volume
- A single log group per environment simplifies log discovery and auditing
- Retention is fixed at 30 days

## Multi-Account Exclusion

Multi-account FIS experiments are **out of scope** for this module. The module is designed for single-account experiments only. The `experiment_options.account_targeting` field defaults to `"single-account"` and is passed through to the provider when explicitly set.

## Deferred Tag-Gating

Mandatory target tag-gating enforcement (e.g., requiring a `managedByFIS` tag on all targets) is **deferred in this release**. Optional target tag selectors are accepted via `resource_tags` in the template input schema, but no fixed opt-in tag is required or enforced.

## Minimal Validation Philosophy

This module implements minimal module-level validation. The Terraform AWS provider and AWS FIS API are the authoritative validators for:

- Action/target compatibility (e.g., valid `action_id` for a given `resource_type`)
- Detailed parameter correctness (e.g., duration format, parameter names)
- Resource type validity
- Cross-field validation within experiment templates

The module validates only what it must at the module boundary:

- Non-empty required inputs (`environment`, `ci_commit_ref_name`)
- S3-safe characters in `ci_commit_ref_name`
- S3 bucket name length (≤63 characters)
- `selection_mode` format and bounds (`COUNT(n) > 0`, `PERCENT(n) 1–100`)
- Mutual exclusivity of `resource_arns` / `resource_tags` per target
- Non-empty `resource_tags` key/value entries


<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.0 |
| aws | >= 5.0 |

## Providers

| Name | Version |
|------|---------|
| aws | >= 5.0 |

## Modules

| Name | Source | Description |
|------|--------|-------------|
| fis\_kms | `artifactory.example.com/terraform-modules/kms` | KMS key for S3 bucket encryption |
| fis\_s3 | `artifactory.example.com/terraform-modules/s3` | S3 bucket for Lambda fault-injection config artifacts |

## Resources

| Name | Type |
|------|------|
| [aws_cloudwatch_log_group.fis_experiments](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudwatch_log_group) | resource |
| [aws_fis_experiment_template.this](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/fis_experiment_template) | resource |
| [aws_caller_identity.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/caller_identity) | data source |
| [aws_iam_role.fis_experiment_role](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/iam_role) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| environment | Environment name used in template naming and log group path | `string` | n/a | yes |
| ci\_commit\_ref\_name | GitLab CI/CD branch/tag ref used in S3 bucket naming | `string` | n/a | yes |
| experiment\_templates | Map of FIS experiment template definitions | `map(object({...}))` | n/a | yes |

### `experiment_templates` Schema

Each entry in the `experiment_templates` map accepts the following attributes:

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| description | `string` | `""` | Template description |
| actions | `map(object({...}))` | n/a (required) | Map of FIS actions (see below) |
| targets | `map(object({...}))` | `{}` | Map of FIS targets (optional for targetless actions) |
| stop\_conditions | `list(object({source, value}))` | `[{source = "none", value = ""}]` | Stop conditions; defaults to `source = "none"` |
| tags | `map(string)` | `{}` | Additional tags for the template resource |
| experiment\_options | `object({account_targeting, empty_target_resolution_mode})` | `null` | Optional experiment options |
| experiment\_report\_configuration | `object({outputs, data_sources, pre_experiment_duration, post_experiment_duration})` | `null` | Optional experiment report configuration |

#### Action Schema

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| action\_id | `string` | n/a (required) | FIS action type ID (e.g., `aws:s3:bucket-pause-replication`) |
| description | `string` | `""` | Action description |
| start\_after | `set(string)` | `[]` | Action dependency ordering |
| targets | `list(object({key, value}))` | `[]` | Target references for this action |
| parameters | `list(object({key, value}))` | `[]` | Action-specific parameters |

#### Target Schema

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| resource\_type | `string` | n/a (required) | FIS resource type (e.g., `aws:s3:bucket`) |
| selection\_mode | `string` | `"ALL"` | `ALL`, `COUNT(n)`, or `PERCENT(n)` |
| resource\_arns | `list(string)` | `[]` | Target resource ARNs (mutually exclusive with `resource_tags`) |
| resource\_tags | `list(object({key, value}))` | `[]` | Target resource tags (mutually exclusive with `resource_arns`) |
| filters | `list(object({path, values}))` | `[]` | Target filters |
| parameters | `map(string)` | `{}` | Target-level parameters |

## Outputs

| Name | Description |
|------|-------------|
| experiment\_role\_arn | Resolved ARN of FISExperimentRole |
| s3\_bucket\_name | Name of the Lambda config S3 bucket |
| s3\_bucket\_arn | ARN of the Lambda config S3 bucket |
| kms\_key\_id | ID of the KMS key used for S3 encryption |
| kms\_key\_arn | ARN of the KMS key |
| experiment\_templates | Map of created experiment template metadata (`id`, `arn`, `name`) keyed by template key |
| log\_group\_name | Name of the shared CloudWatch log group |
| log\_group\_arn | ARN of the shared CloudWatch log group |
<!-- END_TF_DOCS -->

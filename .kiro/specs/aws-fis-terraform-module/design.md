# Design Document: AWS FIS Terraform Module

## Overview

This design describes a Terraform module (`FIS_Module`) that provisions AWS Fault Injection Service experiment templates and supporting infrastructure. The module:

- Creates `aws_fis_experiment_template` resources from a structured, provider-aligned input map (core experiment definition, experiment options, and experiment reporting)
- Provisions a single KMS-encrypted S3 bucket (via internal `S3_Module` and `KMS_Module`) for Lambda fault-injection config artifacts
- Creates a single shared CloudWatch Logs log group for experiment execution logs
- Looks up the pre-existing `FISExperimentRole` IAM role via data source — never creates IAM resources
- Accepts target resource identifiers as inputs — never creates workload resources

The module enforces minimal validation at the module level. The Terraform AWS provider and AWS FIS API are the authoritative validators for action/target compatibility and parameter correctness. The one exception is `Selection_Mode` bounds checking (`COUNT(n) > 0`, `PERCENT(n) 1–100`). The module is provider-aligned for core experiment definition including experiment options and experiment reporting.

### Key Design Decisions

| Decision | Rationale |
|---|---|
| Provider-aligned input schema (core experiment definition) | Reduces mapping complexity; users familiar with `aws_fis_experiment_template` can adopt quickly. Covers actions, targets, stop conditions, log configuration, experiment options, and experiment reporting. |
| Hardcoded `FISExperimentRole` | Single-role convention simplifies lookup; no IAM creation in module scope |
| Shared CloudWatch log group | Low FIS log volume; one log group per environment is sufficient |
| Minimal module-level validation | Provider/API catch most errors with better messages; module validates only what it must |
| Internal S3/KMS modules from Artifactory | Organizational compliance; no direct `aws_s3_bucket` or `aws_kms_key` resources |
| Tag-gating deferred | Documented as future work; optional tag selectors accepted but not enforced |
| `experiment_report_configuration` optional passthrough | Experiment reporting (S3 destination, CloudWatch dashboard data sources, pre/post experiment duration) is supported as an optional block. Callers can configure S3 report output and data sources per template when needed. |
| Optional targets in template schema | Mirrors provider behavior; supports targetless actions (e.g., `aws:fis:wait`) used in multi-action orchestration |
| No multi-account support | Scoped to single-account experiments only |

## IAM Prerequisite Contract

The `FISExperimentRole` IAM role is a mandatory prerequisite that must exist before this module can be used. This module never creates IAM resources — the role is managed in a separate IAM project.

### Required Role Configuration

| Aspect | Requirement |
|---|---|
| Role Name | `FISExperimentRole` (hardcoded lookup) |
| Trust Policy | Must trust `fis.amazonaws.com` as a service principal |
| FIS Action Permissions | Must allow FIS actions on S3, Kinesis, DynamoDB, Lambda, and Network targets |
| S3 Access | Must allow access to the Lambda config bucket provisioned by this module |
| KMS Access | Must allow use of the KMS encryption key provisioned by this module |
| CloudWatch Logs Access | Must allow writing to the shared experiment log group (`/aws/fis/experiments/{environment}`) |

### Trust Policy (Minimum)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Service": "fis.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### Permission Scope

The role must have policies granting:

1. **FIS experiment actions** — permissions for the specific FIS fault-injection actions used in experiment templates (e.g., `s3:PauseBucketReplication`, `kinesis:*`, `dynamodb:*`, `lambda:*`, `ec2:*` as needed per experiment scope)
2. **S3 access** — read/write to the `fis-lambda-config-*` bucket for Lambda fault-injection configuration artifacts
3. **KMS access** — `kms:Decrypt`, `kms:GenerateDataKey` on the module-provisioned KMS key for S3 encryption
4. **CloudWatch Logs access** — `logs:CreateLogStream`, `logs:PutLogEvents` on the shared log group ARN

### Failure Mode

If the role does not exist or lacks the required trust policy, `data.aws_iam_role.fis_experiment_role` will fail during `terraform plan`/`apply` with a clear error. If the role exists but lacks sufficient permissions, experiments will fail at runtime with AWS FIS API errors.

## Architecture

### High-Level Module Diagram

```mermaid
graph TD
    subgraph "FIS_Module"
        direction TB
        VARS[variables.tf<br/>Module Inputs]
        MAIN[main.tf<br/>Provider, Data Sources<br/>aws_caller_identity, aws_region, aws_iam_role]
        KMS[kms.tf<br/>KMS_Module reference]
        S3[s3.tf<br/>S3_Module reference]
        LOGS[logs.tf<br/>CloudWatch Log Group]
        FIS[fis_templates.tf<br/>Experiment Templates]
        OUT[outputs.tf<br/>Module Outputs]
    end

    subgraph "External / Pre-existing"
        IAM_ROLE["IAM: FISExperimentRole"]
        TARGETS["Target Resources<br/>(S3, Kinesis, DynamoDB,<br/>Lambda, Network)"]
    end

    subgraph "Artifactory Internal Modules"
        S3_MOD["S3_Module"]
        KMS_MOD["KMS_Module"]
    end

    VARS --> MAIN
    MAIN -->|data.aws_iam_role| IAM_ROLE
    MAIN -->|data.aws_caller_identity| S3
    KMS --> KMS_MOD
    S3 --> S3_MOD
    S3 -->|encryption key| KMS
    LOGS --> FIS
    MAIN -->|role_arn| FIS
    VARS -->|experiment_templates| FIS
    VARS -->|target identifiers| TARGETS
    FIS --> OUT
    S3 --> OUT
    KMS --> OUT
    LOGS --> OUT
    MAIN --> OUT
```


### Data Flow

```mermaid
sequenceDiagram
    participant User as Module Caller
    participant Vars as variables.tf
    participant Main as main.tf
    participant KMS as kms.tf (KMS_Module)
    participant S3 as s3.tf (S3_Module)
    participant Logs as logs.tf
    participant FIS as fis_templates.tf
    participant Out as outputs.tf

    User->>Vars: environment, ci_commit_ref_name, experiment_templates
    Vars->>Main: validated inputs
    Main->>Main: data.aws_caller_identity → account_id
    Main->>Main: data.aws_region → region name (for ARN construction)
    Main->>Main: data.aws_iam_role("FISExperimentRole") → role_arn
    Main->>KMS: provision KMS key
    KMS-->>S3: kms_key_arn
    Main->>S3: bucket_name = "fis-lambda-config-${account_id}-${ci_commit_ref_name}"
    Main->>Logs: log_group = "/aws/fis/experiments/${environment}"
    Vars->>FIS: experiment_templates map
    Main-->>FIS: role_arn
    Logs-->>FIS: log_group_arn
    FIS->>FIS: for_each over experiment_templates → aws_fis_experiment_template
    FIS-->>Out: template IDs, constructed ARNs, names
    S3-->>Out: bucket name, ARN
    KMS-->>Out: key ID, ARN
    Logs-->>Out: log group name, ARN
    Main-->>Out: role ARN
```

## Components and Interfaces

### File Layout

| File | Responsibility |
|---|---|
| `main.tf` | Provider configuration, `data.aws_caller_identity`, `data.aws_region`, `data.aws_iam_role` lookup |
| `variables.tf` | All module input variables with types, defaults, descriptions, and validation blocks |
| `kms.tf` | `KMS_Module` invocation for S3 encryption key |
| `s3.tf` | `S3_Module` invocation for Lambda config bucket |
| `logs.tf` | `aws_cloudwatch_log_group` for shared experiment logs |
| `fis_templates.tf` | `aws_fis_experiment_template` resources via `for_each` |
| `outputs.tf` | All module outputs |

### Module Inputs (`variables.tf`)

#### Required Inputs

| Variable | Type | Description |
|---|---|---|
| `environment` | `string` | Environment name used in template naming and log group path |
| `ci_commit_ref_name` | `string` | GitLab CI/CD branch/tag ref used in S3 bucket naming |
| `experiment_templates` | `map(object({...}))` | Map of experiment template definitions (see schema below) |

> **Caller sanitization note**: GitLab branch names often contain characters invalid for S3 bucket names (uppercase, slashes, underscores). Callers (e.g., GitLab CI pipelines) should sanitize `CI_COMMIT_REF_NAME` before passing it to the module:
> ```bash
> echo "$CI_COMMIT_REF_NAME" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]/-/g' | sed 's/--*/-/g' | sed 's/^-//;s/-$//'
> ```

#### `experiment_templates` Object Schema

```hcl
variable "experiment_templates" {
  description = "Map of FIS experiment template definitions"
  type = map(object({
    description = optional(string, "")
    
    # Actions block — maps directly to aws_fis_experiment_template action blocks
    actions = map(object({
      action_id   = string                       # FIS action type ID (e.g., "aws:s3:bucket-pause-replication")
      description = optional(string, "")
      start_after = optional(set(string), [])    # action dependency ordering (set of action names)

      # target is a nested block with key/value pairs, NOT a map attribute.
      # Each entry maps a target type key (e.g., "Buckets") to a target name defined in this template.
      targets = optional(list(object({
        key   = string                           # target type key (e.g., "Instances", "Buckets")
        value = string                           # references a target name within this template
      })), [])

      # parameter is a nested block with key/value pairs, NOT a map attribute.
      # Each entry passes an action-specific parameter (e.g., duration).
      parameters = optional(list(object({
        key   = string                           # parameter name (e.g., "duration")
        value = string                           # parameter value (e.g., "PT5M")
      })), [])
    }))

    # Targets block — maps directly to aws_fis_experiment_template target blocks
    # targets is optional to support targetless actions like aws:fis:wait; defaults to empty map
    targets = optional(map(object({
      resource_type  = string                    # FIS resource type (e.g., "aws:s3:bucket")
      selection_mode = optional(string, "ALL")   # ALL | COUNT(n) | PERCENT(n)
      resource_arns  = optional(list(string), [])

      # resource_tag is a nested block with key/value pairs, NOT map(string).
      # Conflicts with resource_arns — only one of resource_arns or resource_tags may be used.
      resource_tags = optional(list(object({
        key   = string
        value = string
      })), [])

      filters = optional(list(object({
        path   = string
        values = list(string)
      })), [])

      parameters = optional(map(string), {})     # optional target-level parameters (map attribute)
    })), {})

    # Stop conditions
    stop_conditions = optional(list(object({
      source = string                            # "none" or "aws:cloudwatch:alarm"
      value  = optional(string, "")              # alarm ARN when source is cloudwatch
    })), [{ source = "none", value = "" }])

    # Tags for the template resource itself
    tags = optional(map(string), {})

    # Experiment options — optional block for account targeting and empty target resolution
    experiment_options = optional(object({
      account_targeting            = optional(string, "single-account")
      empty_target_resolution_mode = optional(string, "fail")
    }), null)

    # Experiment report configuration — optional block for S3 report output and CloudWatch data sources
    experiment_report_configuration = optional(object({
      outputs = optional(object({
        s3_configuration = optional(object({
          bucket_name = string
          prefix      = optional(string, "")
        }), null)
      }), null)
      data_sources = optional(object({
        cloudwatch_dashboards = optional(list(object({
          dashboard_identifier = string
        })), [])
      }), null)
      pre_experiment_duration  = optional(string, null)   # ISO 8601 duration e.g. "PT5M"
      post_experiment_duration = optional(string, null)   # ISO 8601 duration e.g. "PT5M"
    }), null)
  }))

  validation {
    condition = alltrue([
      for tpl_key, tpl in var.experiment_templates : alltrue([
        for tgt_key, tgt in tpl.targets :
          !(length(tgt.resource_arns) > 0 && length(tgt.resource_tags) > 0)
      ]) if length(tpl.targets) > 0
    ])
    error_message = "A target cannot specify both resource_arns and resource_tags. Use one or the other."
  }

  validation {
    condition = alltrue([
      for tpl_key, tpl in var.experiment_templates : alltrue([
        for tgt_key, tgt in tpl.targets : alltrue([
          for tag in tgt.resource_tags :
            trimspace(tag.key) != "" && trimspace(tag.value) != ""
        ])
      ]) if length(tpl.targets) > 0
    ])
    error_message = "Each resource_tag entry must have a non-empty key and a non-empty value."
  }
}
```

#### Template Naming

Each template is named `fis-{service}-{scenario}-{environment}` where the map key encodes `{service}-{scenario}`. The module constructs the full name by appending `-{environment}`.

### Module Outputs (`outputs.tf`)

| Output | Type | Description |
|---|---|---|
| `experiment_role_arn` | `string` | Resolved ARN of `FISExperimentRole` |
| `s3_bucket_name` | `string` | Name of the Lambda config S3 bucket |
| `s3_bucket_arn` | `string` | ARN of the Lambda config S3 bucket |
| `kms_key_id` | `string` | ID of the KMS key used for S3 encryption |
| `kms_key_arn` | `string` | ARN of the KMS key |
| `experiment_templates` | `map(object({id, arn, name}))` | Map of created template metadata keyed by template key. `arn` is constructed from `id` using `arn:aws:fis:{region}:{account_id}:experiment-template/{id}` because the `aws_fis_experiment_template` resource only exports `id`, not `arn`. |
| `log_group_name` | `string` | Name of the shared CloudWatch log group |
| `log_group_arn` | `string` | ARN of the shared CloudWatch log group |


### Component Details

#### `main.tf` — Provider and Data Sources

```hcl
data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

data "aws_iam_role" "fis_experiment_role" {
  name = "FISExperimentRole"
}
```

- `aws_caller_identity` resolves `account_id` for S3 bucket naming — no user input needed.
- `aws_region` resolves the current region for constructing experiment template ARNs (the `aws_fis_experiment_template` resource only exports `id`, not `arn`).
- `aws_iam_role` lookup fails with a clear Terraform error if the role doesn't exist, satisfying Requirement 3.3.

#### `kms.tf` — KMS Key via Internal Module

```hcl
module "fis_kms" {
  source = "artifactory.example.com/terraform-modules/kms"
  # Module-specific arguments per internal KMS_Module interface
  description = "KMS key for FIS Lambda config S3 bucket"
  tags        = { Environment = var.environment }
}
```

#### `s3.tf` — S3 Bucket via Internal Module

```hcl
module "fis_s3" {
  source = "artifactory.example.com/terraform-modules/s3"
  
  bucket_name    = "fis-lambda-config-${data.aws_caller_identity.current.account_id}-${var.ci_commit_ref_name}"
  kms_key_arn    = module.fis_kms.key_arn
  # Additional S3_Module arguments per internal interface
  tags           = { Environment = var.environment }
}
```

Bucket name validation (≤63 chars) is enforced via a `validation` block on the computed name in `variables.tf` or a `locals` + `check` block.

#### `logs.tf` — Shared CloudWatch Log Group

```hcl
resource "aws_cloudwatch_log_group" "fis_experiments" {
  name              = "/aws/fis/experiments/${var.environment}"
  retention_in_days = 30
  tags              = { Environment = var.environment }
}
```

Single log group shared across all templates. Retention is fixed at 30 days per Requirement 5.7.

#### `fis_templates.tf` — Experiment Templates

```hcl
resource "aws_fis_experiment_template" "this" {
  for_each = var.experiment_templates

  description = each.value.description
  role_arn    = data.aws_iam_role.fis_experiment_role.arn

  dynamic "action" {
    for_each = each.value.actions
    content {
      name        = action.key
      action_id   = action.value.action_id
      description = action.value.description

      dynamic "target" {
        for_each = action.value.targets
        content {
          key   = target.value.key
          value = target.value.value
        }
      }

      start_after = action.value.start_after

      dynamic "parameter" {
        for_each = action.value.parameters
        content {
          key   = parameter.value.key
          value = parameter.value.value
        }
      }
    }
  }

  dynamic "target" {
    for_each = each.value.targets
    content {
      name           = target.key
      resource_type  = target.value.resource_type
      selection_mode = target.value.selection_mode
      resource_arns  = length(target.value.resource_arns) > 0 ? target.value.resource_arns : null

      dynamic "resource_tag" {
        for_each = target.value.resource_tags
        content {
          key   = resource_tag.value.key
          value = resource_tag.value.value
        }
      }

      dynamic "filter" {
        for_each = target.value.filters
        content {
          path   = filter.value.path
          values = filter.value.values
        }
      }

      parameters = length(target.value.parameters) > 0 ? target.value.parameters : null
    }
  }

  dynamic "stop_condition" {
    for_each = length(each.value.stop_conditions) > 0 ? each.value.stop_conditions : [{ source = "none", value = "" }]
    content {
      source = stop_condition.value.source
      value  = stop_condition.value.value != "" ? stop_condition.value.value : null
    }
  }

  dynamic "experiment_options" {
    for_each = each.value.experiment_options != null ? [each.value.experiment_options] : []
    content {
      account_targeting            = experiment_options.value.account_targeting
      empty_target_resolution_mode = experiment_options.value.empty_target_resolution_mode
    }
  }

  dynamic "experiment_report_configuration" {
    for_each = each.value.experiment_report_configuration != null ? [each.value.experiment_report_configuration] : []
    content {
      dynamic "outputs" {
        for_each = experiment_report_configuration.value.outputs != null ? [experiment_report_configuration.value.outputs] : []
        content {
          dynamic "s3_configuration" {
            for_each = outputs.value.s3_configuration != null ? [outputs.value.s3_configuration] : []
            content {
              bucket_name = s3_configuration.value.bucket_name
              prefix      = s3_configuration.value.prefix
            }
          }
        }
      }
      dynamic "data_sources" {
        for_each = experiment_report_configuration.value.data_sources != null ? [experiment_report_configuration.value.data_sources] : []
        content {
          dynamic "cloudwatch_dashboards" {
            for_each = data_sources.value.cloudwatch_dashboards
            content {
              dashboard_identifier = cloudwatch_dashboards.value.dashboard_identifier
            }
          }
        }
      }
      pre_experiment_duration  = experiment_report_configuration.value.pre_experiment_duration
      post_experiment_duration = experiment_report_configuration.value.post_experiment_duration
    }
  }

  log_configuration {
    cloudwatch_logs_configuration {
      # aws_cloudwatch_log_group.arn already includes the :* suffix required by FIS
      log_group_arn = aws_cloudwatch_log_group.fis_experiments.arn
    }
    log_schema_version = 2
  }

  tags = merge(
    { Name = "fis-${each.key}-${var.environment}" },
    each.value.tags
  )
}
```

The `each.key` in the `experiment_templates` map encodes `{service}-{scenario}`, so the template `Name` tag becomes `fis-{service}-{scenario}-{environment}`.

#### Constructed Template ARNs

The `aws_fis_experiment_template` resource only exports `id`, not `arn`. The module constructs ARNs in a local:

```hcl
locals {
  experiment_template_arns = {
    for key, tpl in aws_fis_experiment_template.this :
    key => "arn:aws:fis:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:experiment-template/${tpl.id}"
  }
}
```

This is consumed by `outputs.tf` to include `arn` in the `experiment_templates` output map.

### Validation Logic

The module implements minimal validation:

1. **`environment`** — `validation { condition = length(var.environment) > 0 }` (non-empty)
2. **`ci_commit_ref_name`** — multiple validation blocks:
   - Non-empty: `validation { condition = length(var.ci_commit_ref_name) > 0 }`
   - S3-safe characters only (lowercase letters, numbers, hyphens): `validation { condition = can(regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?$", var.ci_commit_ref_name)) }` — rejects uppercase, underscores, slashes, periods, and other characters that violate S3 naming rules or cause DNS/TLS issues
   - No consecutive hyphens: `validation { condition = !can(regex("--", var.ci_commit_ref_name)) }` — prevents consecutive hyphens which can conflict with S3 reserved suffixes
3. **S3 bucket name length** — validated via `locals` computation + `check` or `precondition` (≤63 chars)
4. **`selection_mode`** — custom validation logic:

```hcl
locals {
  # Parse selection_mode into components for validation — avoids brittle regex(...)[0] capture indexing
  selection_mode_checks = flatten([
    for tpl_key, tpl in var.experiment_templates : [
      for tgt_key, tgt in tpl.targets : {
        key        = "${tpl_key}.${tgt_key}"
        mode       = tgt.selection_mode
        is_all     = tgt.selection_mode == "ALL"
        is_count   = can(regex("^COUNT\\(\\d+\\)$", tgt.selection_mode))
        is_percent = can(regex("^PERCENT\\(\\d+\\)$", tgt.selection_mode))
        # Extract numeric value safely using replace instead of regex capture groups
        numeric_value = (
          can(regex("^COUNT\\(\\d+\\)$", tgt.selection_mode))
            ? tonumber(replace(replace(tgt.selection_mode, "COUNT(", ""), ")", ""))
          : can(regex("^PERCENT\\(\\d+\\)$", tgt.selection_mode))
            ? tonumber(replace(replace(tgt.selection_mode, "PERCENT(", ""), ")", ""))
          : null
        )
      }
    ]
  ])

  # Format validation: must be ALL, COUNT(n), or PERCENT(n)
  invalid_selection_mode_formats = [
    for check in local.selection_mode_checks : check.key
    if !(check.is_all || check.is_count || check.is_percent)
  ]

  # Bounds validation: COUNT(n) must have n > 0
  invalid_count_bounds = [
    for check in local.selection_mode_checks : check.key
    if check.is_count && check.numeric_value != null && check.numeric_value <= 0
  ]

  # Bounds validation: PERCENT(n) must have 1 <= n <= 100
  invalid_percent_bounds = [
    for check in local.selection_mode_checks : check.key
    if check.is_percent && check.numeric_value != null && (check.numeric_value < 1 || check.numeric_value > 100)
  ]
}
```

These locals are consumed by `precondition` or `validation` blocks:

- `length(local.invalid_selection_mode_formats) == 0` — rejects unknown formats
- `length(local.invalid_count_bounds) == 0` — rejects `COUNT(0)` or negative
- `length(local.invalid_percent_bounds) == 0` — rejects `PERCENT(0)` or `PERCENT(101+)`

5. **`resource_tags` entries** — validation block ensures each tag has non-empty `key` and `value` (after `trimspace`). Combined with the existing mutual exclusivity check and the `resource_arns` non-empty check, this ensures targets always have meaningful identifiers.

All other validation (action/target compatibility, parameter correctness, resource type validity) is delegated to the Terraform AWS provider and AWS FIS API.

## Data Models

### Experiment Template Input Model

```mermaid
classDiagram
    class ExperimentTemplate {
        +string description
        +map~Action~ actions
        +map~Target~ targets [optional, default empty]
        +list~StopCondition~ stop_conditions
        +map~string~ tags
        +ExperimentOptions experiment_options [optional, default null]
        +ExperimentReportConfiguration experiment_report_configuration [optional, default null]
    }

    class Action {
        +string action_id
        +string description
        +list~ActionTarget~ targets
        +set~string~ start_after
        +list~ActionParameter~ parameters
    }

    class ActionTarget {
        +string key
        +string value
    }

    class ActionParameter {
        +string key
        +string value
    }

    class Target {
        +string resource_type
        +string selection_mode
        +list~string~ resource_arns
        +list~ResourceTag~ resource_tags
        +list~Filter~ filters
        +map~string~ parameters
    }

    class ResourceTag {
        +string key
        +string value
    }

    class Filter {
        +string path
        +list~string~ values
    }

    class StopCondition {
        +string source
        +string value
    }

    class ExperimentOptions {
        +string account_targeting [default: single-account]
        +string empty_target_resolution_mode [default: fail]
    }

    class ExperimentReportConfiguration {
        +ReportOutputs outputs [optional, default null]
        +ReportDataSources data_sources [optional, default null]
        +string pre_experiment_duration [optional, default null]
        +string post_experiment_duration [optional, default null]
    }

    class ReportOutputs {
        +S3Configuration s3_configuration [optional, default null]
    }

    class S3Configuration {
        +string bucket_name
        +string prefix [default: empty]
    }

    class ReportDataSources {
        +list~CloudWatchDashboard~ cloudwatch_dashboards [default: empty]
    }

    class CloudWatchDashboard {
        +string dashboard_identifier
    }

    ExperimentTemplate "1" --> "*" Action : actions
    ExperimentTemplate "1" --> "*" Target : targets
    ExperimentTemplate "1" --> "*" StopCondition : stop_conditions
    ExperimentTemplate "1" --> "0..1" ExperimentOptions : experiment_options
    ExperimentTemplate "1" --> "0..1" ExperimentReportConfiguration : experiment_report_configuration
    Action "1" --> "*" ActionTarget : targets
    Action "1" --> "*" ActionParameter : parameters
    Target "1" --> "*" ResourceTag : resource_tags
    Target "1" --> "*" Filter : filters
    ExperimentReportConfiguration "1" --> "0..1" ReportOutputs : outputs
    ExperimentReportConfiguration "1" --> "0..1" ReportDataSources : data_sources
    ReportOutputs "1" --> "0..1" S3Configuration : s3_configuration
    ReportDataSources "1" --> "*" CloudWatchDashboard : cloudwatch_dashboards
```

**Constraint**: `Target.resource_arns` and `Target.resource_tags` are mutually exclusive — a target must use one or the other, not both.

### Resource Naming Conventions

| Resource | Naming Pattern | Example |
|---|---|---|
| Experiment Template (Name tag) | `fis-{service}-{scenario}-{environment}` | `fis-s3-pause-replication-staging` |
| S3 Bucket | `fis-lambda-config-{account_id}-{ci_commit_ref_name}` | `fis-lambda-config-123456789012-main` |
| CloudWatch Log Group | `/aws/fis/experiments/{environment}` | `/aws/fis/experiments/staging` |
| KMS Key (alias) | Per KMS_Module convention | Module-dependent |

### State and Lifecycle

- All resources are managed by Terraform state — no external state management.
- The S3 bucket, KMS key, and log group are singletons per module invocation.
- Experiment templates are created/updated/destroyed via `for_each` keyed on the `experiment_templates` map.
- Removing a key from the map destroys the corresponding template.
- The IAM role is a data source only — never managed by this module.


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: S3 Bucket Name Construction

*For any* valid `account_id` and `ci_commit_ref_name`, the constructed S3 bucket name SHALL equal `"fis-lambda-config-${account_id}-${ci_commit_ref_name}"`.

**Validates: Requirements 2.5**

### Property 2: S3 Bucket Name Length Validation

*For any* `ci_commit_ref_name` value, if the resulting bucket name `"fis-lambda-config-${account_id}-${ci_commit_ref_name}"` exceeds 63 characters, the module SHALL reject the input with a validation error. If the name is ≤63 characters, the module SHALL accept it.

**Validates: Requirements 2.6**

### Property 3: Template Count Equals Input Count

*For any* `experiment_templates` map with N entries, the module SHALL create exactly N `aws_fis_experiment_template` resources, and the output map SHALL contain exactly N entries.

**Validates: Requirements 4.2, 9.4**

### Property 4: Uniform Template Configuration

*For any* set of experiment templates created by the module, every template SHALL reference the same resolved `Experiment_Role_Arn` and the same shared CloudWatch log group ARN.

**Validates: Requirements 4.3, 5.3**

### Property 5: Stop Condition Default

*For any* experiment template where `stop_conditions` is not provided (or is empty), the created template SHALL have exactly one stop condition with `source = "none"`.

**Validates: Requirements 4.5**

### Property 6: Selection Mode Default

*For any* target within an experiment template where `selection_mode` is not specified, the created target SHALL have `selection_mode = "ALL"`.

**Validates: Requirements 4.7**

### Property 7: Selection Mode Validation

*For any* target `selection_mode` value: if it is `COUNT(n)` then `n` must be an integer > 0; if it is `PERCENT(n)` then `n` must be an integer from 1 through 100; if it is `ALL` it is always valid. Any other format or out-of-bounds value SHALL cause a validation error.

**Validates: Requirements 4.10, 4.11, 4.12**

### Property 8: Template Naming Convention

*For any* experiment template with map key `{service}-{scenario}` and module input `environment`, the template Name tag SHALL equal `"fis-{service}-{scenario}-{environment}"`.

**Validates: Requirements 4.16**

### Property 9: Single Deterministic Log Group

*For any* `environment` value and any number of experiment templates (≥0), the module SHALL create exactly one CloudWatch log group named `"/aws/fis/experiments/${environment}"`.

**Validates: Requirements 5.1, 5.2**

### Property 10: Non-Empty Target Identifier Validation

*For any* target that specifies `resource_arns` directly, the module SHALL validate that the list is non-empty. *For any* target that specifies `resource_tags`, the module SHALL validate that at least one tag entry exists and that each tag entry has a non-empty (after trimming) `key` and `value`. An empty `resource_arns` list (when no `resource_tags` are provided) or `resource_tags` with blank entries SHALL cause a validation error.

**Validates: Requirements 7.2, 7.7**

### Property 11: Output Map Completeness

*For any* experiment template key in the input map, the `experiment_templates` output map SHALL contain a corresponding entry with non-empty `id`, `arn`, and `name` fields. The `arn` is constructed as `arn:aws:fis:{region}:{account_id}:experiment-template/{id}` using `data.aws_region.current.name` and `data.aws_caller_identity.current.account_id`, since the `aws_fis_experiment_template` resource only exports `id`.

**Validates: Requirements 9.4**

### Property 12: ci_commit_ref_name S3-Safe Character Validation

*For any* `ci_commit_ref_name` value, the module SHALL reject values containing characters outside `[a-z0-9-]`, values starting or ending with a hyphen, or values containing consecutive hyphens. Only values matching `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` without consecutive hyphens SHALL be accepted.

**Validates: Requirements 2.7**


## Error Handling

### Module-Level Errors

| Error Condition | Mechanism | Message Guidance |
|---|---|---|
| `FISExperimentRole` IAM role not found | `data.aws_iam_role` lookup failure | Terraform surfaces "role not found" error from AWS API |
| `FISExperimentRole` lacks required permissions | Runtime FIS API error | AWS FIS returns descriptive permission errors at experiment execution time |
| S3 bucket name > 63 characters | `validation` block or `precondition` | `"S3 bucket name exceeds 63 characters. Shorten ci_commit_ref_name."` |
| `environment` is empty | `validation` block on variable | `"environment must not be empty."` |
| `ci_commit_ref_name` is empty | `validation` block on variable | `"ci_commit_ref_name must not be empty."` |
| `ci_commit_ref_name` contains invalid characters | `validation` block on variable | `"ci_commit_ref_name must contain only lowercase letters, numbers, and hyphens, and must not start or end with a hyphen."` |
| `ci_commit_ref_name` contains consecutive hyphens | `validation` block on variable | `"ci_commit_ref_name must not contain consecutive hyphens."` |
| `COUNT(n)` where n ≤ 0 | `precondition` via `local.invalid_count_bounds` | `"COUNT selection_mode requires n > 0."` |
| `PERCENT(n)` where n < 1 or n > 100 | `precondition` via `local.invalid_percent_bounds` | `"PERCENT selection_mode requires n between 1 and 100."` |
| Invalid `selection_mode` format | `precondition` via `local.invalid_selection_mode_formats` | `"selection_mode must be ALL, COUNT(n), or PERCENT(n)."` |
| Target specifies both `resource_arns` and `resource_tags` | `validation` block on `experiment_templates` variable | `"A target cannot specify both resource_arns and resource_tags. Use one or the other."` |
| Empty `resource_arns` with no `resource_tags` | `precondition` on target | `"Target must specify non-empty resource_arns or resource_tags."` |
| `resource_tags` entry has empty key or value | `validation` block on `experiment_templates` variable | `"Each resource_tag entry must have a non-empty key and a non-empty value."` |

### Provider/API-Level Errors (Delegated)

These errors are intentionally not caught at the module level:

- Invalid `action_id` for a given resource type
- Incompatible action/target combinations
- Invalid action parameters (e.g., malformed duration)
- Resource type not supported by FIS
- Cross-field validation within experiment templates

The Terraform AWS provider and AWS FIS API return descriptive errors for these cases. The module does not duplicate this validation.

### Error Propagation Strategy

1. **Plan-time errors**: Variable validation blocks and `precondition` blocks catch errors during `terraform plan`.
2. **Apply-time errors**: Data source lookups (IAM role) and provider-level validation surface during `terraform apply`.
3. **API-time errors**: FIS API rejects invalid experiment configurations with descriptive error messages.

## Testing Strategy

### Dual Testing Approach

The module uses two complementary testing strategies:

1. **Property-based tests** — Verify universal properties across generated inputs using a property-based testing library
2. **Integration tests (Terratest)** — Verify end-to-end behavior against real AWS infrastructure

### Property-Based Testing

**Library**: [rapid](https://github.com/flyingmutant/rapid) (Go property-based testing library, compatible with Terratest's Go ecosystem)

**Configuration**:
- Minimum 100 iterations per property test
- Each test tagged with: `Feature: aws-fis-terraform-module, Property {number}: {property_text}`

**Properties to implement**:

| Property | Test Description |
|---|---|
| Property 1 | Generate random account_id (12-digit) and ci_commit_ref_name strings, verify bucket name matches pattern |
| Property 2 | Generate ci_commit_ref_name of varying lengths, verify validation accepts/rejects based on 63-char limit |
| Property 3 | Generate experiment_templates maps of varying sizes (1-10), verify output count matches input count |
| Property 4 | Generate multiple templates, verify all share the same role_arn and log_group_arn |
| Property 5 | Generate templates with and without stop_conditions, verify default behavior |
| Property 6 | Generate targets with and without selection_mode, verify default is "ALL" |
| Property 7 | Generate selection_mode strings (valid and invalid COUNT/PERCENT values), verify validation accepts/rejects correctly |
| Property 8 | Generate template keys and environment values, verify Name tag follows convention |
| Property 9 | Generate varying numbers of templates with a fixed environment, verify exactly one log group with correct name |
| Property 10 | Generate targets with empty and non-empty resource_arns/resource_tags, verify validation behavior including rejection of blank tag key/value entries |
| Property 11 | Generate template keys, verify output map has matching entries with non-empty id/arn/name; verify arn follows `arn:aws:fis:{region}:{account_id}:experiment-template/{id}` pattern |
| Property 12 | Generate ci_commit_ref_name strings (valid and invalid: uppercase, underscores, slashes, periods, leading/trailing hyphens, consecutive hyphens), verify validation accepts only S3-safe values matching `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` without consecutive hyphens |

**Note**: Properties 1, 2, 7, 8, and 12 can be tested as pure functions (naming/validation logic) without provisioning infrastructure. Properties 3, 4, 5, 6, 9, 10, and 11 require Terraform plan/apply verification and are best validated through Terratest integration tests with property-based input generation.

### Integration Testing (Terratest)

**Framework**: Terratest (Go)

**Test Structure**:

```
tests/
  fis_module_test.go       # Main integration test file
  fixtures/
    main.tf                # Test fixture invoking the FIS module
    variables.tf           # Test input variables
    outputs.tf             # Test outputs for assertions
    terraform.tfvars       # Default test values
```

**Test Scenarios**:

1. **Full module deployment** — Provision one target per service (S3, Kinesis, DynamoDB, Lambda, Network), invoke FIS_Module, validate all outputs
2. **Output validation** — Verify all outputs (role ARN, S3 bucket name/ARN, KMS key ID/ARN, log group name/ARN, template IDs/ARNs/names)
3. **Template creation** — Verify each service target produces a valid experiment template
4. **Role reference** — Verify every template references the resolved `FISExperimentRole` ARN

**Teardown**: Every test uses `defer terraform.Destroy(t, terraformOptions)` as the first action after `terraform.InitAndApply` to ensure cleanup.

**Test fixture setup**:
- Uses internal modules (S3_Module, KMS_Module, Lambda_Module, and equivalents for Kinesis, DynamoDB, Network) to provision one target resource per service
- Passes provisioned resource identifiers to the FIS_Module
- Asserts on FIS_Module outputs

### Test Coverage Matrix

| Requirement | Property Test | Integration Test |
|---|---|---|
| Req 1 (Module structure) | — | Implicit (module loads) |
| Req 2 (S3 bucket) | P1, P2, P12 | Output assertions |
| Req 3 (IAM role) | — | Role ARN output check |
| Req 4 (Templates) | P3, P4, P5, P6, P7, P8 | Template creation per service |
| Req 5 (CloudWatch) | P9 | Log group output check |
| Req 6 (No multi-account) | — | — (absence verified by design) |
| Req 7 (Target inputs) | P10 | Target resource pass-through |
| Req 8 (Tag-gating deferred) | — | — (documented only) |
| Req 9 (Outputs) | P11 | All output assertions |
| Req 10 (Terratest) | — | Full integration test |

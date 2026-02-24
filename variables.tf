variable "environment" {
  description = "Environment name used in template naming and log group path"
  type        = string

  validation {
    condition     = length(var.environment) > 0
    error_message = "environment must not be empty."
  }
}

variable "ci_commit_ref_name" {
  description = "GitLab CI/CD branch/tag ref used in S3 bucket naming"
  type        = string

  validation {
    condition     = length(var.ci_commit_ref_name) > 0
    error_message = "ci_commit_ref_name must not be empty."
  }

  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?$", var.ci_commit_ref_name))
    error_message = "ci_commit_ref_name must contain only lowercase letters, numbers, and hyphens, and must not start or end with a hyphen."
  }

  validation {
    condition     = !can(regex("--", var.ci_commit_ref_name))
    error_message = "ci_commit_ref_name must not contain consecutive hyphens."
  }
}

variable "experiment_templates" {
  description = "Map of FIS experiment template definitions"
  type = map(object({
    description = optional(string, "")

    actions = map(object({
      action_id   = string
      description = optional(string, "")
      start_after = optional(set(string), [])

      targets = optional(list(object({
        key   = string
        value = string
      })), [])

      parameters = optional(list(object({
        key   = string
        value = string
      })), [])
    }))

    targets = optional(map(object({
      resource_type  = string
      selection_mode = optional(string, "ALL")
      resource_arns  = optional(list(string), [])

      resource_tags = optional(list(object({
        key   = string
        value = string
      })), [])

      filters = optional(list(object({
        path   = string
        values = list(string)
      })), [])

      parameters = optional(map(string), {})
    })), {})

    stop_conditions = optional(list(object({
      source = string
      value  = optional(string, "")
    })), [{ source = "none", value = "" }])

    tags = optional(map(string), {})

    experiment_options = optional(object({
      account_targeting            = optional(string, "single-account")
      empty_target_resolution_mode = optional(string, "fail")
    }), null)

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
      pre_experiment_duration  = optional(string, null)
      post_experiment_duration = optional(string, null)
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

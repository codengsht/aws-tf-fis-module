locals {
  # Parse selection_mode into components for validation
  selection_mode_checks = flatten([
    for tpl_key, tpl in var.experiment_templates : [
      for tgt_key, tgt in tpl.targets : {
        key        = "${tpl_key}.${tgt_key}"
        mode       = tgt.selection_mode
        is_all     = tgt.selection_mode == "ALL"
        is_count   = can(regex("^COUNT\\(\\d+\\)$", tgt.selection_mode))
        is_percent = can(regex("^PERCENT\\(\\d+\\)$", tgt.selection_mode))
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

  invalid_selection_mode_formats = [
    for check in local.selection_mode_checks : check.key
    if !(check.is_all || check.is_count || check.is_percent)
  ]

  invalid_count_bounds = [
    for check in local.selection_mode_checks : check.key
    if check.is_count && check.numeric_value != null && check.numeric_value <= 0
  ]

  invalid_percent_bounds = [
    for check in local.selection_mode_checks : check.key
    if check.is_percent && check.numeric_value != null && (check.numeric_value < 1 || check.numeric_value > 100)
  ]
}

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
      log_group_arn = aws_cloudwatch_log_group.fis_experiments.arn
    }
    log_schema_version = 2
  }

  tags = merge(
    { Name = "fis-${each.key}-${var.environment}" },
    each.value.tags
  )

  lifecycle {
    precondition {
      condition = alltrue([
        for tgt_key, tgt in each.value.targets :
        length(tgt.resource_arns) > 0 || length(tgt.resource_tags) > 0
      ])
      error_message = "Each target must specify non-empty resource_arns or resource_tags."
    }

    precondition {
      condition     = length(local.invalid_selection_mode_formats) == 0
      error_message = "selection_mode must be ALL, COUNT(n), or PERCENT(n). Invalid targets: ${join(", ", local.invalid_selection_mode_formats)}"
    }

    precondition {
      condition     = length(local.invalid_count_bounds) == 0
      error_message = "COUNT selection_mode requires n > 0. Invalid targets: ${join(", ", local.invalid_count_bounds)}"
    }

    precondition {
      condition     = length(local.invalid_percent_bounds) == 0
      error_message = "PERCENT selection_mode requires n between 1 and 100. Invalid targets: ${join(", ", local.invalid_percent_bounds)}"
    }
  }
}

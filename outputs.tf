locals {
  experiment_template_arns = {
    for key, tpl in aws_fis_experiment_template.this :
    key => "arn:aws:fis:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:experiment-template/${tpl.id}"
  }
}

output "experiment_role_arn" {
  description = "Resolved ARN of FISExperimentRole"
  value       = data.aws_iam_role.fis_experiment_role.arn
}

output "s3_bucket_name" {
  description = "Name of the Lambda config S3 bucket"
  value       = module.fis_s3.bucket_name
}

output "s3_bucket_arn" {
  description = "ARN of the Lambda config S3 bucket"
  value       = module.fis_s3.bucket_arn
}

output "kms_key_id" {
  description = "ID of the KMS key used for S3 encryption"
  value       = module.fis_kms.key_id
}

output "kms_key_arn" {
  description = "ARN of the KMS key"
  value       = module.fis_kms.key_arn
}

output "experiment_templates" {
  description = "Map of created experiment template metadata keyed by template key"
  value = {
    for key, tpl in aws_fis_experiment_template.this : key => {
      id   = tpl.id
      arn  = local.experiment_template_arns[key]
      name = "fis-${key}-${var.environment}"
    }
  }
}

output "log_group_name" {
  description = "Name of the shared CloudWatch log group"
  value       = aws_cloudwatch_log_group.fis_experiments.name
}

output "log_group_arn" {
  description = "ARN of the shared CloudWatch log group"
  value       = aws_cloudwatch_log_group.fis_experiments.arn
}

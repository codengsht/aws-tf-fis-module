# -----------------------------------------------------------------------------
# Expose FIS_Module outputs for Terratest assertions
# -----------------------------------------------------------------------------

output "experiment_role_arn" {
  description = "Resolved ARN of FISExperimentRole"
  value       = module.fis.experiment_role_arn
}

output "s3_bucket_name" {
  description = "Name of the Lambda config S3 bucket"
  value       = module.fis.s3_bucket_name
}

output "s3_bucket_arn" {
  description = "ARN of the Lambda config S3 bucket"
  value       = module.fis.s3_bucket_arn
}

output "kms_key_id" {
  description = "ID of the KMS key used for S3 encryption"
  value       = module.fis.kms_key_id
}

output "kms_key_arn" {
  description = "ARN of the KMS key"
  value       = module.fis.kms_key_arn
}

output "experiment_templates" {
  description = "Map of created experiment template metadata"
  value       = module.fis.experiment_templates
}

output "log_group_name" {
  description = "Name of the shared CloudWatch log group"
  value       = module.fis.log_group_name
}

output "log_group_arn" {
  description = "ARN of the shared CloudWatch log group"
  value       = module.fis.log_group_arn
}

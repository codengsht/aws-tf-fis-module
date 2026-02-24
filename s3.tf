locals {
  s3_bucket_name = "fis-lambda-config-${data.aws_caller_identity.current.account_id}-${var.ci_commit_ref_name}"
}

check "s3_bucket_name_length" {
  assert {
    condition     = length(local.s3_bucket_name) <= 63
    error_message = "S3 bucket name exceeds 63 characters. Shorten ci_commit_ref_name."
  }
}

module "fis_s3" {
  source = "artifactory.example.com/terraform-modules/s3"

  bucket_name = local.s3_bucket_name
  kms_key_arn = module.fis_kms.key_arn
  tags        = { Environment = var.environment }
}

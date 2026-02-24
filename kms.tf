module "fis_kms" {
  source = "artifactory.example.com/terraform-modules/kms"

  description = "KMS key for FIS Lambda config S3 bucket"
  tags        = { Environment = var.environment }
}

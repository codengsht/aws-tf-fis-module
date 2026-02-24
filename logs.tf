resource "aws_cloudwatch_log_group" "fis_experiments" {
  name              = "/aws/fis/experiments/${var.environment}"
  retention_in_days = 30
  tags              = { Environment = var.environment }
}

# -----------------------------------------------------------------------------
# Test Fixtures: Provision target resources externally, then invoke FIS_Module
# -----------------------------------------------------------------------------
# The FIS_Module does NOT create Kinesis, DynamoDB, Lambda, or network
# infrastructure. Those resources are provisioned here using internal modules
# and passed to the FIS_Module as target identifiers.
# -----------------------------------------------------------------------------

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = "eu-west-1"
}

# ---------------------
# Data sources
# ---------------------
data "aws_caller_identity" "current" {}

# ---------------------
# Target: S3 bucket
# ---------------------
module "target_s3" {
  source = "artifactory.example.com/terraform-modules/s3"

  bucket_name = "fis-test-target-${data.aws_caller_identity.current.account_id}"
  tags        = { Environment = var.environment }
}

# ---------------------
# Target: Kinesis stream
# ---------------------
module "target_kinesis" {
  source = "artifactory.example.com/terraform-modules/kinesis"

  stream_name = "fis-test-target-stream"
  shard_count = 1
  tags        = { Environment = var.environment }
}

# ---------------------
# Target: DynamoDB table
# ---------------------
module "target_dynamodb" {
  source = "artifactory.example.com/terraform-modules/dynamodb"

  table_name = "fis-test-target-table"
  hash_key   = "id"
  tags       = { Environment = var.environment }
}

# ---------------------
# Target: Lambda function
# ---------------------
module "target_lambda" {
  source = "artifactory.example.com/terraform-modules/lambda"

  function_name = "fis-test-target-function"
  runtime       = "python3.12"
  handler       = "index.handler"
  tags          = { Environment = var.environment }
}

# ---------------------
# Target: Network (VPC)
# ---------------------
module "target_network" {
  source = "artifactory.example.com/terraform-modules/network"

  vpc_name = "fis-test-target-vpc"
  tags     = { Environment = var.environment }
}

# ---------------------
# FIS Module under test
# ---------------------
module "fis" {
  source = "../../"

  environment        = var.environment
  ci_commit_ref_name = var.ci_commit_ref_name

  experiment_templates = {
    # S3: pause bucket replication
    "s3-pause-replication" = {
      description = "Pause S3 bucket replication"
      actions = {
        "pause-replication" = {
          action_id   = "aws:s3:bucket-pause-replication"
          description = "Pause replication on target S3 bucket"
          targets = [{
            key   = "Buckets"
            value = "target-s3-bucket"
          }]
          parameters = [{
            key   = "duration"
            value = "PT5M"
          }]
        }
      }
      targets = {
        "target-s3-bucket" = {
          resource_type  = "aws:s3:bucket"
          selection_mode = "ALL"
          resource_arns  = [module.target_s3.bucket_arn]
        }
      }
      stop_conditions = [{ source = "none" }]
      tags            = { Service = "s3" }
    }

    # Kinesis: deactivate stream consumer
    "kinesis-deactivate-stream" = {
      description = "Deactivate Kinesis stream consumer"
      actions = {
        "deactivate-consumer" = {
          action_id   = "aws:kinesis:deactivate-stream-consumer"
          description = "Deactivate consumer on target Kinesis stream"
          targets = [{
            key   = "StreamConsumers"
            value = "target-kinesis-stream"
          }]
          parameters = [{
            key   = "duration"
            value = "PT5M"
          }]
        }
      }
      targets = {
        "target-kinesis-stream" = {
          resource_type  = "aws:kinesis:stream-consumer"
          selection_mode = "ALL"
          resource_arns  = [module.target_kinesis.stream_arn]
        }
      }
      stop_conditions = [{ source = "none" }]
      tags            = { Service = "kinesis" }
    }

    # DynamoDB: pause replication
    "dynamodb-pause-replication" = {
      description = "Pause DynamoDB global table replication"
      actions = {
        "pause-replication" = {
          action_id   = "aws:dynamodb:global-table-pause-replication"
          description = "Pause replication on target DynamoDB table"
          targets = [{
            key   = "Tables"
            value = "target-dynamodb-table"
          }]
          parameters = [{
            key   = "duration"
            value = "PT5M"
          }]
        }
      }
      targets = {
        "target-dynamodb-table" = {
          resource_type  = "aws:dynamodb:global-table"
          selection_mode = "ALL"
          resource_arns  = [module.target_dynamodb.table_arn]
        }
      }
      stop_conditions = [{ source = "none" }]
      tags            = { Service = "dynamodb" }
    }

    # Lambda: inject fault
    "lambda-inject-fault" = {
      description = "Inject fault into Lambda function"
      actions = {
        "inject-fault" = {
          action_id   = "aws:lambda:invocation-add-delay"
          description = "Add invocation delay to target Lambda function"
          targets = [{
            key   = "Functions"
            value = "target-lambda-function"
          }]
          parameters = [
            { key = "duration", value = "PT5M" },
            { key = "delayMilliseconds", value = "500" }
          ]
        }
      }
      targets = {
        "target-lambda-function" = {
          resource_type  = "aws:lambda:function"
          selection_mode = "ALL"
          resource_arns  = [module.target_lambda.function_arn]
        }
      }
      stop_conditions = [{ source = "none" }]
      tags            = { Service = "lambda" }
    }

    # Network: disrupt connectivity
    "network-disrupt-connectivity" = {
      description = "Disrupt network connectivity for EC2 instances"
      actions = {
        "disrupt-connectivity" = {
          action_id   = "aws:ec2:send-spot-instance-interruptions"
          description = "Disrupt connectivity on target EC2 instances"
          targets = [{
            key   = "SpotInstances"
            value = "target-ec2-instances"
          }]
          parameters = [{
            key   = "durationBeforeInterruption"
            value = "PT2M"
          }]
        }
      }
      targets = {
        "target-ec2-instances" = {
          resource_type  = "aws:ec2:spot-instance"
          selection_mode = "ALL"
          resource_tags = [{
            key   = "Environment"
            value = var.environment
          }]
        }
      }
      stop_conditions = [{ source = "none" }]
      tags            = { Service = "network" }
    }
  }
}

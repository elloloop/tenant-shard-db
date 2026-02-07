# -----------------------------------------------------------------------------
# EntDB Infrastructure
#
# Deploys EntDB on AWS with:
# - S3 bucket for snapshots and WAL archives
# - Kinesis stream (default) or MSK cluster for WAL
# - ECS Fargate service for the EntDB server
# -----------------------------------------------------------------------------

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = merge(var.tags, {
      Project     = var.project
      Environment = var.environment
      ManagedBy   = "terraform"
    })
  }
}

locals {
  name_prefix = "${var.project}-${var.environment}"
  use_kinesis = var.wal_backend == "kinesis"
  use_msk     = var.wal_backend == "msk"
}

# -- Data sources -------------------------------------------------------------

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

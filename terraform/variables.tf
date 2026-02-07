# -----------------------------------------------------------------------------
# EntDB Terraform Variables
# -----------------------------------------------------------------------------

variable "project" {
  description = "Project name used for resource naming"
  type        = string
  default     = "entdb"
}

variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

# -- WAL Backend --------------------------------------------------------------

variable "wal_backend" {
  description = "WAL backend: kinesis (default, cheaper) or msk"
  type        = string
  default     = "kinesis"

  validation {
    condition     = contains(["kinesis", "msk"], var.wal_backend)
    error_message = "wal_backend must be 'kinesis' or 'msk'."
  }
}

# -- Kinesis ------------------------------------------------------------------

variable "kinesis_shard_count" {
  description = "Number of shards for Kinesis stream (provisioned mode)"
  type        = number
  default     = 1
}

variable "kinesis_retention_hours" {
  description = "Data retention in hours (24-8760)"
  type        = number
  default     = 24
}

# -- MSK (optional) ----------------------------------------------------------

variable "msk_instance_type" {
  description = "MSK broker instance type"
  type        = string
  default     = "kafka.m7g.large"
}

variable "msk_broker_count" {
  description = "Number of MSK broker nodes (must be multiple of AZ count)"
  type        = number
  default     = 3
}

variable "msk_ebs_volume_size" {
  description = "EBS volume size per broker in GB"
  type        = number
  default     = 50
}

# -- S3 ----------------------------------------------------------------------

variable "s3_force_destroy" {
  description = "Allow terraform destroy to delete non-empty S3 bucket"
  type        = bool
  default     = false
}

# -- ECS ---------------------------------------------------------------------

variable "entdb_image" {
  description = "Docker image for EntDB server (e.g. ghcr.io/elloloop/tenant-shard-db:v0.1.0)"
  type        = string
  default     = "ghcr.io/elloloop/tenant-shard-db:latest"
}

variable "entdb_cpu" {
  description = "Fargate CPU units (256, 512, 1024, 2048, 4096)"
  type        = number
  default     = 512
}

variable "entdb_memory" {
  description = "Fargate memory in MiB"
  type        = number
  default     = 1024
}

variable "entdb_desired_count" {
  description = "Number of ECS tasks to run"
  type        = number
  default     = 1
}

variable "entdb_grpc_port" {
  description = "gRPC server port"
  type        = number
  default     = 50051
}

# -- Networking ---------------------------------------------------------------

variable "vpc_id" {
  description = "VPC ID (if empty, creates a new VPC)"
  type        = string
  default     = ""
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for ECS tasks (if empty, creates new subnets)"
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}

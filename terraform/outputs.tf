# -----------------------------------------------------------------------------
# Outputs
# -----------------------------------------------------------------------------

output "s3_bucket_name" {
  description = "S3 bucket for snapshots and WAL archives"
  value       = aws_s3_bucket.data.id
}

output "s3_bucket_arn" {
  description = "S3 bucket ARN"
  value       = aws_s3_bucket.data.arn
}

# -- WAL outputs --------------------------------------------------------------

output "wal_backend" {
  description = "Active WAL backend"
  value       = var.wal_backend
}

output "kinesis_stream_name" {
  description = "Kinesis stream name (when using Kinesis)"
  value       = local.use_kinesis ? aws_kinesis_stream.wal[0].name : null
}

output "kinesis_stream_arn" {
  description = "Kinesis stream ARN (when using Kinesis)"
  value       = local.use_kinesis ? aws_kinesis_stream.wal[0].arn : null
}

output "msk_bootstrap_brokers" {
  description = "MSK bootstrap brokers (when using MSK)"
  value       = local.use_msk ? aws_msk_cluster.wal[0].bootstrap_brokers : null
}

output "msk_cluster_arn" {
  description = "MSK cluster ARN (when using MSK)"
  value       = local.use_msk ? aws_msk_cluster.wal[0].arn : null
}

# -- ECS outputs --------------------------------------------------------------

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.this.name
}

output "ecs_service_name" {
  description = "ECS service name"
  value       = aws_ecs_service.entdb.name
}

output "ecs_task_role_arn" {
  description = "IAM role ARN for ECS tasks"
  value       = aws_iam_role.ecs_task.arn
}

output "cloudwatch_log_group" {
  description = "CloudWatch log group for EntDB"
  value       = aws_cloudwatch_log_group.entdb.name
}

# -- Estimated cost -----------------------------------------------------------

output "estimated_monthly_cost" {
  description = "Rough monthly cost estimate (USD)"
  value = local.use_kinesis ? (
    "~$50-80/mo (Kinesis $26 + ECS Fargate ~$20-50 + S3 minimal)"
  ) : (
    "~$500-600/mo (MSK $452 + ECS Fargate ~$20-50 + S3 minimal)"
  )
}

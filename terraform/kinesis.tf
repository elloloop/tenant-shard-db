# -----------------------------------------------------------------------------
# Kinesis Data Stream for WAL (default backend, ~$26/month for 1 shard)
# -----------------------------------------------------------------------------

resource "aws_kinesis_stream" "wal" {
  count = local.use_kinesis ? 1 : 0

  name             = "${local.name_prefix}-wal"
  shard_count      = var.kinesis_shard_count
  retention_period = var.kinesis_retention_hours

  stream_mode_details {
    stream_mode = "PROVISIONED"
  }

  encryption_type = "KMS"
  kms_key_id      = "alias/aws/kinesis"

  tags = { Name = "${local.name_prefix}-wal" }
}

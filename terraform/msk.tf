# -----------------------------------------------------------------------------
# MSK Cluster for WAL (optional, use wal_backend="msk")
# ~$452/month minimum for 3x kafka.m7g.large
# -----------------------------------------------------------------------------

resource "aws_msk_cluster" "wal" {
  count = local.use_msk ? 1 : 0

  cluster_name           = "${local.name_prefix}-wal"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = var.msk_broker_count

  broker_node_group_info {
    instance_type   = var.msk_instance_type
    client_subnets  = local.private_subnet_ids
    security_groups = [aws_security_group.msk[0].id]

    storage_info {
      ebs_storage_info {
        volume_size = var.msk_ebs_volume_size
      }
    }
  }

  encryption_info {
    encryption_in_transit {
      client_broker = "TLS_PLAINTEXT"
      in_cluster    = true
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.wal[0].arn
    revision = aws_msk_configuration.wal[0].latest_revision
  }

  tags = { Name = "${local.name_prefix}-wal" }
}

resource "aws_msk_configuration" "wal" {
  count = local.use_msk ? 1 : 0

  name              = "${local.name_prefix}-wal-config"
  kafka_versions    = ["3.6.0"]
  server_properties = <<-PROPERTIES
    auto.create.topics.enable=true
    default.replication.factor=3
    min.insync.replicas=2
    num.partitions=3
    log.retention.hours=${var.kinesis_retention_hours}
    log.segment.bytes=1073741824
  PROPERTIES
}

resource "aws_security_group" "msk" {
  count = local.use_msk ? 1 : 0

  name_prefix = "${local.name_prefix}-msk-"
  vpc_id      = local.vpc_id
  description = "MSK broker security group"

  ingress {
    from_port       = 9092
    to_port         = 9098
    protocol        = "tcp"
    security_groups = [aws_security_group.entdb.id]
    description     = "Kafka from EntDB"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }

  tags = { Name = "${local.name_prefix}-msk-sg" }
}

# -----------------------------------------------------------------------------
# ECS Fargate Service for EntDB Server
# -----------------------------------------------------------------------------

resource "aws_ecs_cluster" "this" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_cloudwatch_log_group" "entdb" {
  name              = "/ecs/${local.name_prefix}"
  retention_in_days = 30
}

resource "aws_ecs_task_definition" "entdb" {
  family                   = "${local.name_prefix}-server"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.entdb_cpu
  memory                   = var.entdb_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([
    {
      name      = "entdb"
      image     = var.entdb_image
      essential = true

      portMappings = [{
        containerPort = var.entdb_grpc_port
        protocol      = "tcp"
      }]

      environment = concat(
        [
          { name = "ENTDB_GRPC_PORT", value = tostring(var.entdb_grpc_port) },
          { name = "ENTDB_DATA_DIR", value = "/data" },
          { name = "ENTDB_S3_BUCKET", value = aws_s3_bucket.data.id },
          { name = "ENTDB_S3_REGION", value = var.aws_region },
          { name = "ENTDB_S3_PREFIX_SNAPSHOT", value = "snapshots" },
          { name = "ENTDB_S3_PREFIX_ARCHIVE", value = "wal-archive" },
        ],
        local.use_kinesis ? [
          { name = "ENTDB_WAL_BACKEND", value = "kinesis" },
          { name = "ENTDB_KINESIS_STREAM", value = aws_kinesis_stream.wal[0].name },
          { name = "ENTDB_KINESIS_REGION", value = var.aws_region },
        ] : [],
        local.use_msk ? [
          { name = "ENTDB_WAL_BACKEND", value = "kafka" },
          { name = "ENTDB_KAFKA_BROKERS", value = aws_msk_cluster.wal[0].bootstrap_brokers },
          { name = "ENTDB_KAFKA_SECURITY_PROTOCOL", value = "PLAINTEXT" },
        ] : [],
      )

      mountPoints = [{
        sourceVolume  = "data"
        containerPath = "/data"
        readOnly      = false
      }]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.entdb.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "entdb"
        }
      }

      healthCheck = {
        command     = ["CMD-SHELL", "python -c 'import grpc; ch = grpc.insecure_channel(\"localhost:${var.entdb_grpc_port}\"); grpc.channel_ready_future(ch).result(timeout=5)' || exit 1"]
        interval    = 30
        timeout     = 10
        retries     = 3
        startPeriod = 60
      }
    },
  ])

  volume {
    name = "data"
    efs_volume_configuration {
      file_system_id = aws_efs_file_system.data.id
      root_directory = "/"
    }
  }
}

# EFS for persistent SQLite storage
resource "aws_efs_file_system" "data" {
  creation_token = "${local.name_prefix}-data"
  encrypted      = true

  tags = { Name = "${local.name_prefix}-data" }
}

resource "aws_efs_mount_target" "data" {
  count = length(local.private_subnet_ids)

  file_system_id  = aws_efs_file_system.data.id
  subnet_id       = local.private_subnet_ids[count.index]
  security_groups = [aws_security_group.efs.id]
}

resource "aws_security_group" "efs" {
  name_prefix = "${local.name_prefix}-efs-"
  vpc_id      = local.vpc_id
  description = "EFS mount target security group"

  ingress {
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = [aws_security_group.entdb.id]
    description     = "NFS from EntDB tasks"
  }

  tags = { Name = "${local.name_prefix}-efs-sg" }
}

resource "aws_ecs_service" "entdb" {
  name            = "${local.name_prefix}-server"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.entdb.arn
  desired_count   = var.entdb_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = local.private_subnet_ids
    security_groups = [aws_security_group.entdb.id]
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}

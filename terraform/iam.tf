# -----------------------------------------------------------------------------
# IAM Roles and Policies for EntDB ECS Tasks
# -----------------------------------------------------------------------------

# -- ECS Task Execution Role (pulls images, writes logs) ----------------------

resource "aws_iam_role" "ecs_execution" {
  name = "${local.name_prefix}-ecs-execution"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_execution" {
  role       = aws_iam_role.ecs_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# -- ECS Task Role (S3, Kinesis/MSK access) -----------------------------------

resource "aws_iam_role" "ecs_task" {
  name = "${local.name_prefix}-ecs-task"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

# S3 access for snapshots and archives
resource "aws_iam_role_policy" "s3_access" {
  name = "${local.name_prefix}-s3-access"
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:ListBucket",
          "s3:DeleteObject",
        ]
        Resource = [
          aws_s3_bucket.data.arn,
          "${aws_s3_bucket.data.arn}/*",
        ]
      },
    ]
  })
}

# Kinesis access (when using Kinesis backend)
resource "aws_iam_role_policy" "kinesis_access" {
  count = local.use_kinesis ? 1 : 0

  name = "${local.name_prefix}-kinesis-access"
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kinesis:PutRecord",
          "kinesis:PutRecords",
          "kinesis:GetRecords",
          "kinesis:GetShardIterator",
          "kinesis:DescribeStream",
          "kinesis:DescribeStreamSummary",
          "kinesis:ListShards",
          "kinesis:SubscribeToShard",
        ]
        Resource = aws_kinesis_stream.wal[0].arn
      },
    ]
  })
}

# MSK/Kafka access (when using MSK backend)
resource "aws_iam_role_policy" "msk_access" {
  count = local.use_msk ? 1 : 0

  name = "${local.name_prefix}-msk-access"
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "kafka-cluster:Connect",
          "kafka-cluster:DescribeTopic",
          "kafka-cluster:ReadData",
          "kafka-cluster:WriteData",
          "kafka-cluster:CreateTopic",
          "kafka-cluster:DescribeGroup",
          "kafka-cluster:AlterGroup",
        ]
        Resource = [
          aws_msk_cluster.wal[0].arn,
          "${aws_msk_cluster.wal[0].arn}/*",
        ]
      },
    ]
  })
}

# Deployment Guide

This guide covers deploying EntDB to production environments.

## Architecture Overview

Production deployment consists of:

```
┌─────────────────────────────────────────────────────────────────┐
│                         Load Balancer                            │
│                    (ALB with gRPC support)                       │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                        ECS Service                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                       │
│  │ EntDB    │  │ EntDB    │  │ EntDB    │  (Auto-scaling)       │
│  │ Server 1 │  │ Server 2 │  │ Server 3 │                       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                       │
│       │             │             │                              │
│       └─────────────┼─────────────┘                              │
│                     │                                            │
└─────────────────────┼────────────────────────────────────────────┘
                      │
┌─────────────────────▼────────────────────────────────────────────┐
│                   Amazon MSK (Kafka)                              │
│              (3+ brokers, multi-AZ)                               │
└──────────────────────────────────────────────────────────────────┘
                      │
┌─────────────────────▼────────────────────────────────────────────┐
│                      Amazon S3                                    │
│        (Archive + Snapshots + Terraform State)                    │
└──────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- AWS account with appropriate permissions
- Terraform >= 1.5
- Docker
- AWS CLI configured

## Infrastructure Setup

### 1. Create Terraform Configuration

```hcl
# terraform/main.tf

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket = "your-terraform-state"
    key    = "entdb/terraform.tfstate"
    region = "us-east-1"
  }
}

provider "aws" {
  region = var.aws_region
}

# VPC
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"

  name = "entdb-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["us-east-1a", "us-east-1b", "us-east-1c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  enable_nat_gateway = true
  single_nat_gateway = false
}

# MSK Cluster
resource "aws_msk_cluster" "entdb" {
  cluster_name           = "entdb-wal"
  kafka_version          = "3.5.1"
  number_of_broker_nodes = 3

  broker_node_group_info {
    instance_type   = "kafka.m5.large"
    client_subnets  = module.vpc.private_subnets
    security_groups = [aws_security_group.msk.id]

    storage_info {
      ebs_storage_info {
        volume_size = 100
      }
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.entdb.arn
    revision = aws_msk_configuration.entdb.latest_revision
  }
}

resource "aws_msk_configuration" "entdb" {
  name = "entdb-config"

  server_properties = <<PROPERTIES
auto.create.topics.enable=false
min.insync.replicas=2
default.replication.factor=3
unclean.leader.election.enable=false
PROPERTIES
}

# S3 Buckets
resource "aws_s3_bucket" "archive" {
  bucket = "entdb-archive-${var.environment}"
}

resource "aws_s3_bucket" "snapshots" {
  bucket = "entdb-snapshots-${var.environment}"
}

# ECS Cluster
resource "aws_ecs_cluster" "entdb" {
  name = "entdb-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

# ECS Task Definition
resource "aws_ecs_task_definition" "entdb" {
  family                   = "entdb-server"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 1024
  memory                   = 2048
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([
    {
      name  = "entdb"
      image = "ghcr.io/your-org/entdb:${var.image_tag}"

      portMappings = [
        {
          containerPort = 50051
          protocol      = "tcp"
        },
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]

      environment = [
        {
          name  = "ENTDB_KAFKA_BROKERS"
          value = aws_msk_cluster.entdb.bootstrap_brokers_tls
        },
        {
          name  = "ENTDB_S3_BUCKET"
          value = aws_s3_bucket.archive.id
        },
        {
          name  = "ENTDB_SNAPSHOT_BUCKET"
          value = aws_s3_bucket.snapshots.id
        }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = "/ecs/entdb"
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }

      healthCheck = {
        command     = ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }
    }
  ])
}

# ECS Service
resource "aws_ecs_service" "entdb" {
  name            = "entdb-service"
  cluster         = aws_ecs_cluster.entdb.id
  task_definition = aws_ecs_task_definition.entdb.arn
  desired_count   = 3
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = module.vpc.private_subnets
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.grpc.arn
    container_name   = "entdb"
    container_port   = 50051
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.http.arn
    container_name   = "entdb"
    container_port   = 8080
  }
}

# Application Load Balancer
resource "aws_lb" "entdb" {
  name               = "entdb-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = module.vpc.public_subnets
}

resource "aws_lb_target_group" "grpc" {
  name        = "entdb-grpc"
  port        = 50051
  protocol    = "HTTP"
  protocol_version = "GRPC"
  vpc_id      = module.vpc.vpc_id
  target_type = "ip"

  health_check {
    enabled             = true
    path                = "/grpc.health.v1.Health/Check"
    matcher             = "0"
    interval            = 30
  }
}

resource "aws_lb_target_group" "http" {
  name        = "entdb-http"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = module.vpc.vpc_id
  target_type = "ip"

  health_check {
    path = "/health"
  }
}
```

### 2. Deploy Infrastructure

```bash
cd terraform

# Initialize
terraform init

# Plan
terraform plan -var="environment=production" -var="image_tag=v1.0.0"

# Apply
terraform apply -var="environment=production" -var="image_tag=v1.0.0"
```

## Container Image

### Build and Push

```bash
# Build image
docker build -t ghcr.io/your-org/entdb:v1.0.0 --target server .

# Push to GHCR
docker push ghcr.io/your-org/entdb:v1.0.0

# Or use CI/CD (automatic on tag)
git tag v1.0.0
git push origin v1.0.0
```

### Multi-Architecture

The CI/CD pipeline builds for both AMD64 and ARM64:

```yaml
# .github/workflows/release.yml
platforms: linux/amd64,linux/arm64
```

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ENTDB_KAFKA_BROKERS` | Yes | Kafka bootstrap servers |
| `ENTDB_KAFKA_ACKS` | No | Producer acks (default: all) |
| `ENTDB_S3_BUCKET` | Yes | S3 bucket for archive |
| `ENTDB_SNAPSHOT_BUCKET` | No | S3 bucket for snapshots |
| `ENTDB_DATA_DIR` | No | Local data directory |
| `ENTDB_GRPC_PORT` | No | gRPC port (default: 50051) |
| `ENTDB_HTTP_PORT` | No | HTTP port (default: 8080) |

### Secrets

Store sensitive values in AWS Secrets Manager:

```bash
aws secretsmanager create-secret \
  --name entdb/production/kafka \
  --secret-string '{"username":"admin","password":"secret"}'
```

Reference in task definition:

```json
{
  "name": "ENTDB_KAFKA_PASSWORD",
  "valueFrom": "arn:aws:secretsmanager:region:account:secret:entdb/production/kafka:password::"
}
```

## Scaling

### Horizontal Scaling

ECS service auto-scaling based on CPU:

```hcl
resource "aws_appautoscaling_target" "entdb" {
  max_capacity       = 10
  min_capacity       = 3
  resource_id        = "service/${aws_ecs_cluster.entdb.name}/${aws_ecs_service.entdb.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "cpu" {
  name               = "entdb-cpu"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.entdb.resource_id
  scalable_dimension = aws_appautoscaling_target.entdb.scalable_dimension
  service_namespace  = aws_appautoscaling_target.entdb.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value = 70
  }
}
```

### Kafka Scaling

MSK broker scaling:

```bash
# Increase broker count (only increase, cannot decrease)
aws kafka update-cluster-configuration \
  --cluster-arn $CLUSTER_ARN \
  --current-version $VERSION \
  --configuration-info file://new-config.json
```

## Monitoring

### CloudWatch Dashboards

Create dashboard with key metrics:

- ECS CPU/Memory utilization
- Request latency (p50, p95, p99)
- Error rates
- Kafka consumer lag
- S3 archive lag

### Alarms

```hcl
resource "aws_cloudwatch_metric_alarm" "high_latency" {
  alarm_name          = "entdb-high-latency"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "TargetResponseTime"
  namespace           = "AWS/ApplicationELB"
  period              = 60
  statistic           = "p99"
  threshold           = 1
  alarm_description   = "Request latency exceeds 1 second"

  dimensions = {
    LoadBalancer = aws_lb.entdb.arn_suffix
  }

  alarm_actions = [aws_sns_topic.alerts.arn]
}
```

## Backup and Recovery

### Automated Snapshots

Configure snapshot schedule:

```bash
ENTDB_SNAPSHOT_ENABLED=true
ENTDB_SNAPSHOT_INTERVAL_HOURS=6
ENTDB_SNAPSHOT_RETENTION_DAYS=30
```

### Manual Snapshot

```bash
# Trigger manual snapshot
curl -X POST http://entdb:8080/admin/snapshot

# List snapshots
aws s3 ls s3://entdb-snapshots/tenant_123/
```

### Disaster Recovery

See [Durability Guide](durability.md) for recovery procedures.

## Security

### Network Security

- VPC with private subnets
- Security groups restrict access
- TLS for all connections

### IAM Roles

Least-privilege IAM roles:

```hcl
resource "aws_iam_role_policy" "ecs_task" {
  role = aws_iam_role.ecs_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.archive.arn,
          "${aws_s3_bucket.archive.arn}/*",
          aws_s3_bucket.snapshots.arn,
          "${aws_s3_bucket.snapshots.arn}/*"
        ]
      }
    ]
  })
}
```

### Encryption

- S3 buckets with SSE-S3 or SSE-KMS
- MSK with TLS and encryption at rest
- Secrets in Secrets Manager

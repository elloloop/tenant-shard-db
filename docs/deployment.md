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
      ]
      # gRPC-only server; no HTTP listener. The EntDB Console (HTTP)
      # is a separate binary deployed as its own ECS service if you
      # want it in production.

      # The Go server takes CLI flags only — no ENTDB_* env vars.
      command = [
        "-addr=:50051",
        "-data-dir=/var/lib/entdb",
        "-wal-backend=kafka",
        "-wal-brokers=${aws_msk_cluster.entdb.bootstrap_brokers_tls}",
        "-wal-topic=entdb-wal",
        "-wal-group=entdb-applier",
        "-require-tls=true",
        "-tls-cert=/etc/entdb/server.crt",
        "-tls-key=/etc/entdb/server.key",
        "-kms-provider=aws",
        "-kms-key-id=${aws_kms_key.entdb_master.arn}",
        "-encryption-required=true",
        "-archive-enabled=true",
        "-archive-bucket=${aws_s3_bucket.archive.id}",
        "-archive-region=${var.aws_region}",
        "-archive-retention-days=2557",
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = "/ecs/entdb"
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }

      # No container-level healthcheck: the Go server image is
      # distroless (no shell, no curl, no grpc_health_probe). Health
      # is asserted by the ALB target group on the gRPC port via
      # /grpc.health.v1.Health/Check (see aws_lb_target_group.grpc).
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

# The Go server has no HTTP listener — no `entdb-http` target group is
# defined. Health is asserted via the gRPC target group above
# (path = "/grpc.health.v1.Health/Check"). If you also deploy the
# entdb-console binary, give it its own target group on :8080.
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

## Tenant Onboarding

The Go server does not auto-create tenants, users, or memberships —
every tenant and every actor that performs writes must be onboarded
explicitly through `Admin.CreateUser` → `Admin.CreateTenant` →
`Admin.AddTenantMember` from an admin/system caller. Wire your identity
service to call these RPCs whenever a new customer or principal needs
to be provisioned.

See [Onboarding](onboarding.md) for the worked example, the admin-actor
production wiring (OAuth/API-key/session), and the idempotency contract.

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

The Go server takes **CLI flags only** — no `ENTDB_*` env vars. Production launches should include the security flags below:

```bash
/entdb-server \
  -addr=:50051 \
  -data-dir=/var/lib/entdb \
  -wal-backend=kafka \
  -wal-brokers="$KAFKA_BROKERS" \
  -tls-cert=/etc/entdb/tls/server.pem \
  -tls-key=/etc/entdb/tls/server-key.pem \
  -tls-ca=/etc/entdb/tls/client-ca.pem \
  -require-tls=true \
  -require-client-cert=true \
  -kms-provider=aws \
  -kms-key-id="$AWS_KMS_KEY_ID" \
  -encryption-required=true
```

`-kms-provider=file` accepts `-kms-key-id=/path/to/master.key` or
`-kms-key-id=env:ENTDB_MASTER_KEY` for local development. The key
material must be 32 raw bytes, 64 hex characters, or base64-encoded
32 bytes. `-kms-provider=aws` uses AWS KMS envelope encryption: the
server generates one 256-bit data key on first boot, persists only the
KMS ciphertext blob under `-data-dir`, and decrypts it on later boots.
`-kms-provider=vault` uses Vault Transit data keys with `VAULT_ADDR`,
`VAULT_TOKEN`, and optional `VAULT_NAMESPACE`.

### Kafka transport security (SASL / TLS)

Authenticated brokers (Confluent Cloud, MSK, self-managed Kafka, Azure
Event Hubs over the Kafka protocol) need TLS and/or SASL. These flags
apply to both the producer and consumer connections:

```bash
/entdb-server \
  -wal-backend=kafka \
  -wal-brokers="$KAFKA_BROKERS" \
  -wal-kafka-tls=true \
  -wal-kafka-sasl-mechanism=PLAIN \           # or SCRAM-SHA-256 / SCRAM-SHA-512
  -wal-kafka-sasl-username="$KAFKA_USER" \
  -wal-kafka-sasl-password="$KAFKA_PASSWORD"
```

- `-wal-kafka-tls` enables TLS (the "SSL" in `SASL_SSL`). With no CA
  file the host's system roots are used — correct for managed brokers.
  `-wal-kafka-tls-ca-file` adds a private CA bundle;
  `-wal-kafka-tls-cert-file` + `-wal-kafka-tls-key-file` enable mutual
  TLS. `-wal-kafka-tls-insecure` skips verification (testing only).
- `-wal-kafka-sasl-mechanism` selects `PLAIN` (Confluent Cloud API
  key/secret, Event Hubs connection string) or `SCRAM-SHA-256` /
  `SCRAM-SHA-512` (self-managed, MSK SCRAM). Leaving it empty disables
  SASL. For `SASL_SSL` set both `-wal-kafka-tls=true` and a mechanism.

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
          "s3:ListBucket",
          # Boot-time verification: the archiver calls
          # GetObjectLockConfiguration and refuses to start unless the
          # bucket is Object Lock COMPLIANCE (ADR-015).
          "s3:GetBucketObjectLockConfiguration",
          # Required so the durable legal-hold-lift worker can lift the
          # Object Lock legal hold on a released tenant's already-archived
          # objects (EPIC #511 Gap 1; SetLegalHold OFF durably enqueues,
          # the worker sweeps). Does NOT grant retention changes —
          # COMPLIANCE retain-until stays immutable (ADR-015).
          "s3:GetObjectLegalHold",
          "s3:PutObjectLegalHold"
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

The archive bucket has hard requirements — Object Lock enabled **at
creation** (it cannot be added later), versioning, and a COMPLIANCE-mode
default retention matching `-archive-retention-days`. The archiver verifies
this at boot and refuses to start otherwise. The bucket-creation steps and
the archive lag / legal-hold-lift ops runbook live in the
[Operations Guide](operations.md#s3-object-lock-archive); the same content
renders in the docs site under Compliance → Audit & Object Lock. Add
`kms:GenerateDataKey` + `kms:Decrypt` on the key only when you set
`-archive-kms-key-id`. EntDB ships **no** Terraform/IaC module for the
bucket — the `aws_s3_bucket.archive` resource above is an illustrative
example; how you provision it is your choice (see
[ADR-015](adr/015-wal-and-s3-object-lock-as-audit-log.md)).

### Encryption

- EntDB SQLite files with SQLCipher (`-encryption-required=true`)
- S3 buckets with SSE-S3 or SSE-KMS
- MSK with TLS and encryption at rest
- Secrets in Secrets Manager

`global.db` and every `tenant_<id>.db` file are opened with SQLCipher
when a master key is configured. Existing plaintext files are refused
when encryption is configured, so migrate data before enabling
`-encryption-required=true` on an existing plaintext deployment.

### gRPC TLS and mTLS

Run plaintext only for local development. In production set
`-require-tls=true`; set `-require-client-cert=true` when service
callers authenticate with client certificates. The server verifies
client certificates against `-tls-ca` and maps the verified URI SAN,
DNS SAN, email SAN, or CommonName into a trusted `system:<id>` actor
for request authorization.

Certificates may be issued by ACME, a cloud-managed private CA, or an
internal CA. Keep the certificate, key, and CA bundle on disk and send
`SIGHUP` to the process after rotation; new TLS handshakes use the
reloaded cert/key/CA without restarting the server.

# -----------------------------------------------------------------------------
# VPC (created only when vpc_id is not provided)
# -----------------------------------------------------------------------------

resource "aws_vpc" "this" {
  count = var.vpc_id == "" ? 1 : 0

  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = { Name = "${local.name_prefix}-vpc" }
}

locals {
  vpc_id = var.vpc_id != "" ? var.vpc_id : aws_vpc.this[0].id
  azs    = slice(data.aws_availability_zones.available.names, 0, 3)

  private_subnet_ids = length(var.private_subnet_ids) > 0 ? var.private_subnet_ids : aws_subnet.private[*].id
}

resource "aws_subnet" "private" {
  count = var.vpc_id == "" ? 3 : 0

  vpc_id            = local.vpc_id
  cidr_block        = cidrsubnet("10.0.0.0/16", 8, count.index + 10)
  availability_zone = local.azs[count.index]

  tags = { Name = "${local.name_prefix}-private-${local.azs[count.index]}" }
}

# VPC endpoints for private subnets (ECR, S3, CloudWatch)
resource "aws_vpc_endpoint" "s3" {
  count = var.vpc_id == "" ? 1 : 0

  vpc_id       = local.vpc_id
  service_name = "com.amazonaws.${var.aws_region}.s3"

  tags = { Name = "${local.name_prefix}-s3-endpoint" }
}

resource "aws_security_group" "entdb" {
  name_prefix = "${local.name_prefix}-entdb-"
  vpc_id      = local.vpc_id
  description = "EntDB server security group"

  ingress {
    from_port   = var.entdb_grpc_port
    to_port     = var.entdb_grpc_port
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
    description = "gRPC from VPC"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }

  tags = { Name = "${local.name_prefix}-entdb-sg" }
}

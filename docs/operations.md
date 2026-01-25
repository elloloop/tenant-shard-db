# Operations Guide

Guide for operating EntDB in production.

## Monitoring

### Key Metrics

#### Request Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `entdb_requests_total` | Total requests by endpoint | - |
| `entdb_request_duration_seconds` | Request latency histogram | p99 > 1s |
| `entdb_errors_total` | Error count by type | > 10/min |

#### WAL Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `entdb_wal_append_duration_seconds` | WAL write latency | p99 > 100ms |
| `entdb_wal_consumer_lag` | Consumer lag in events | > 10000 |
| `entdb_wal_events_total` | Total events produced | - |

#### SQLite Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `entdb_sqlite_query_duration_seconds` | Query latency | p99 > 500ms |
| `entdb_sqlite_connections_active` | Active connections | > pool size |
| `entdb_sqlite_size_bytes` | Database file size | > 10GB |

#### Applier Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `entdb_applier_lag_seconds` | Time behind WAL | > 60s |
| `entdb_applier_events_applied_total` | Events applied | - |
| `entdb_applier_errors_total` | Apply errors | > 0 |

#### Archive/Snapshot Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `entdb_archive_lag_events` | Events not yet archived | > 10000 |
| `entdb_snapshot_age_seconds` | Time since last snapshot | > 24h |
| `entdb_s3_upload_duration_seconds` | S3 upload latency | p99 > 30s |

### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'entdb'
    static_configs:
      - targets: ['entdb:9090']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Grafana Dashboard

Import dashboard JSON from `deploy/grafana/entdb-dashboard.json`.

Key panels:
- Request rate and latency
- Error rate by type
- WAL producer/consumer lag
- Applier processing rate
- SQLite query performance
- Resource utilization

### CloudWatch Integration

```python
# Enable CloudWatch metrics
ENTDB_CLOUDWATCH_ENABLED=true
ENTDB_CLOUDWATCH_NAMESPACE=EntDB
ENTDB_CLOUDWATCH_REGION=us-east-1
```

## Logging

### Log Configuration

```python
# Environment variables
ENTDB_LOG_LEVEL=INFO
ENTDB_LOG_FORMAT=json  # or "text"
ENTDB_LOG_OUTPUT=stdout
```

### Log Levels

| Level | Description |
|-------|-------------|
| DEBUG | Detailed debugging info |
| INFO | Normal operations |
| WARNING | Potential issues |
| ERROR | Errors that need attention |
| CRITICAL | System failures |

### Structured Logging

JSON log format:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "message": "Request processed",
  "tenant_id": "my_tenant",
  "actor": "user:alice",
  "request_id": "req_abc123",
  "duration_ms": 45,
  "endpoint": "/v1/atomic"
}
```

### Log Aggregation

CloudWatch Logs with Log Insights:

```
fields @timestamp, @message
| filter level = "ERROR"
| sort @timestamp desc
| limit 100
```

## Backup and Recovery

### Automated Backups

WAL archiving runs continuously:
- Events archived to S3 in JSONL.gz format
- Retention configurable (default 90 days)

SQLite snapshots run periodically:
- Full database backups to S3
- Includes manifest with WAL position
- Retention configurable (default 30 days)

### Manual Backup

```bash
# Trigger manual snapshot
curl -X POST http://localhost:8080/admin/snapshot \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Backup specific tenant
python -m dbaas.entdb_server.tools.snapshot \
  --tenant-id tenant_123 \
  --output-dir /backup
```

### Recovery Procedures

#### Point-in-Time Recovery

```bash
# List available snapshots
python -m dbaas.entdb_server.tools.restore list-snapshots \
  --tenant-id tenant_123

# Restore to specific time
python -m dbaas.entdb_server.tools.restore \
  --tenant-id tenant_123 \
  --target-time "2024-01-15T14:30:00Z" \
  --output-dir /var/lib/entdb/data

# Verify integrity
python -m dbaas.entdb_server.tools.restore verify \
  --data-dir /var/lib/entdb/data
```

#### Full Disaster Recovery

```bash
# 1. Stop applier
# 2. Restore latest snapshot
python -m dbaas.entdb_server.tools.restore from-snapshot \
  --tenant-id tenant_123 \
  --snapshot-id snap_2024-01-15T00:00:00Z \
  --output-dir /var/lib/entdb/data

# 3. Replay archive since snapshot
python -m dbaas.entdb_server.tools.restore replay-archive \
  --tenant-id tenant_123 \
  --from-position 12345 \
  --data-dir /var/lib/entdb/data

# 4. Restart applier
```

### Backup Verification

Weekly verification job:

```bash
#!/bin/bash
# verify-backups.sh

# Pick random tenant
TENANT=$(aws s3 ls s3://entdb-snapshots/ | shuf -n1 | awk '{print $2}')

# Restore to temp location
python -m dbaas.entdb_server.tools.restore \
  --tenant-id "$TENANT" \
  --output-dir /tmp/backup-verify

# Verify integrity
python -m dbaas.entdb_server.tools.restore verify \
  --data-dir /tmp/backup-verify

# Cleanup
rm -rf /tmp/backup-verify
```

## Scaling

### Horizontal Scaling

EntDB servers are stateless and can scale horizontally:

```bash
# ECS service scaling
aws ecs update-service \
  --cluster entdb-cluster \
  --service entdb-service \
  --desired-count 5
```

Auto-scaling based on CPU/memory:

```hcl
resource "aws_appautoscaling_policy" "cpu" {
  name               = "entdb-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = "service/entdb-cluster/entdb-service"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"

  target_tracking_scaling_policy_configuration {
    target_value = 70.0
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
  }
}
```

### Kafka Scaling

Increase partitions for higher throughput:

```bash
kafka-topics.sh --bootstrap-server localhost:9092 \
  --alter --topic entdb-wal \
  --partitions 16
```

Add brokers for capacity:

```bash
# In Terraform
resource "aws_msk_cluster" "entdb" {
  number_of_broker_nodes = 6  # Increased from 3
}
```

### SQLite Optimization

For high-traffic tenants:

```python
# Increase cache size
PRAGMA cache_size = -64000;  # 64MB

# Optimize for read-heavy workloads
PRAGMA read_uncommitted = 1;

# Analyze tables periodically
ANALYZE;
```

## Troubleshooting

### Common Issues

#### High Applier Lag

**Symptoms**: Data appears stale, writes don't show in reads

**Diagnosis**:
```bash
# Check lag metric
curl http://localhost:9090/metrics | grep applier_lag

# Check applier logs
docker logs entdb-applier | grep -i error
```

**Solutions**:
1. Scale applier instances
2. Check Kafka consumer group health
3. Verify SQLite isn't locked
4. Check for slow queries blocking writes

#### Connection Timeouts

**Symptoms**: Clients see timeout errors

**Diagnosis**:
```bash
# Check server health
curl http://localhost:8080/health

# Check connection count
netstat -an | grep 50051 | wc -l

# Check Kafka connectivity
kafka-console-consumer.sh --bootstrap-server localhost:9092 \
  --topic entdb-wal --max-messages 1
```

**Solutions**:
1. Increase connection pool size
2. Scale server instances
3. Check network security groups
4. Verify Kafka broker health

#### Schema Compatibility Errors

**Symptoms**: Deployment fails with schema check error

**Diagnosis**:
```bash
# Show diff
python -m dbaas.entdb_server.tools.schema_cli diff \
  --baseline .schema-snapshot.json \
  --current myapp.schema
```

**Solutions**:
1. Review breaking changes
2. Use migration strategy (see schema-evolution.md)
3. Coordinate with clients before schema changes

### Debug Mode

Enable debug logging:

```bash
ENTDB_LOG_LEVEL=DEBUG docker-compose up
```

Enable request tracing:

```bash
ENTDB_TRACING_ENABLED=true
ENTDB_JAEGER_ENDPOINT=http://jaeger:14268/api/traces
```

### Support

For issues:
1. Check logs for error messages
2. Review metrics for anomalies
3. Search existing issues on GitHub
4. Open new issue with:
   - EntDB version
   - Configuration
   - Steps to reproduce
   - Relevant logs

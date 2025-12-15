# Performance Tuning

This guide covers performance optimization for BlazeLog deployments.

---

## Quick Reference

| Component | Key Setting | Default | High Volume |
|-----------|-------------|---------|-------------|
| Agent batch size | `agent.batch_size` | 100 | 500 |
| Agent flush interval | `agent.flush_interval` | 1s | 2s |
| gRPC workers | - | NumCPU | NumCPU * 2 |
| ClickHouse batch | Internal | 5000 rows | 10000 rows |
| SSH connections/host | `ssh.pool.max_per_host` | 5 | 10 |

---

## Agent Tuning

### Batch Settings

```yaml
# agent.yaml
agent:
  # Entries per batch (higher = more efficient, more latency)
  batch_size: 100

  # Max time to buffer before sending (higher = more efficient, more latency)
  flush_interval: 1s
```

**Recommendations:**

| Log Volume | batch_size | flush_interval |
|------------|------------|----------------|
| Low (<100/s) | 50 | 500ms |
| Medium (100-1000/s) | 100 | 1s |
| High (1000-10000/s) | 500 | 2s |
| Very High (>10000/s) | 1000 | 5s |

### Buffer Memory

Agent memory usage scales with batch size:
- ~1KB per log entry (average)
- 100 batch_size ≈ 100KB buffer
- 1000 batch_size ≈ 1MB buffer

For memory-constrained environments, reduce batch_size.

### Reconnection

On connection loss, agent buffers logs locally:

```yaml
agent:
  # Maximum buffered entries during disconnect (default: 10000)
  max_buffer_entries: 10000
```

Memory impact: `max_buffer_entries * 1KB`

---

## Server Tuning

### gRPC Server

```yaml
# server.yaml
server:
  grpc_address: ":9443"

  # gRPC keepalive (optional)
  grpc:
    max_connection_idle: "5m"
    max_connection_age: "30m"
    keepalive_time: "30s"
    keepalive_timeout: "10s"
```

### HTTP Server

```yaml
server:
  http_address: ":8080"

  # HTTP timeouts (optional)
  http:
    read_timeout: "30s"
    write_timeout: "30s"
    idle_timeout: "120s"
```

### Concurrent Connections

BlazeLog handles multiple agent connections efficiently. For very high agent counts:

- Ensure sufficient file descriptors (`ulimit -n 65535`)
- Consider connection load balancing across multiple servers

---

## Storage Tuning

### ClickHouse Optimization

#### Batch Insert Size

The server batches inserts to ClickHouse:
- Default: 5000 rows or 5 seconds
- For high volume: Increase to 10000-50000 rows

#### Partitioning

Default partitioning is monthly (`toYYYYMM(timestamp)`):

```sql
PARTITION BY toYYYYMM(timestamp)
```

For high-volume deployments, consider daily partitioning:

```sql
PARTITION BY toYYYYMMDD(timestamp)
```

#### Memory Settings

ClickHouse server settings:

```xml
<!-- /etc/clickhouse-server/config.d/memory.xml -->
<clickhouse>
    <max_memory_usage>10000000000</max_memory_usage> <!-- 10GB -->
    <max_memory_usage_for_all_queries>20000000000</max_memory_usage_for_all_queries>
</clickhouse>
```

#### Query Performance

For slow queries, add more indexes:

```sql
-- Add index for frequent filters
ALTER TABLE logs ADD INDEX idx_source source TYPE bloom_filter GRANULARITY 4;
ALTER TABLE logs ADD INDEX idx_type type TYPE bloom_filter GRANULARITY 4;
```

### SQLite Optimization

SQLite is used for configuration (users, alerts, projects). Performance is generally not a concern, but:

```yaml
database:
  # Use WAL mode for better concurrent access (default)
  path: "./data/blazelog.db?_journal_mode=WAL"
```

---

## Alert Engine Tuning

### Evaluation Interval

```yaml
# alerts.yaml
engine:
  # How often to evaluate alert rules (default: 30s)
  interval: 30s
```

| Alert Type | Recommended Interval |
|------------|----------------------|
| Real-time critical | 10s |
| Standard monitoring | 30s |
| Trend analysis | 60s+ |

### Query Optimization

Alert queries run against ClickHouse. Optimize by:
- Using specific `log_type` filters
- Keeping threshold windows short (5m better than 1h)
- Limiting pattern complexity (simpler regex = faster)

---

## SSH Collection Tuning

### Connection Pool

```yaml
# server.yaml
ssh:
  pool:
    max_per_host: 5      # Max connections per remote host
    idle_timeout: 5m     # Close idle connections
```

For high-latency connections:

```yaml
ssh:
  pool:
    max_per_host: 10     # More parallel connections
    idle_timeout: 10m    # Keep connections longer
```

### Large Files

For very large log files, SSH collection may be slower than agents. Consider:
- Using agents for high-volume sources
- Splitting large files with logrotate

---

## Resource Requirements

### Minimum Requirements

| Component | CPU | Memory | Disk |
|-----------|-----|--------|------|
| Server | 2 cores | 2GB | 10GB |
| Agent | 0.5 core | 256MB | 100MB |
| ClickHouse | 2 cores | 4GB | 50GB+ |

### Scaling Guidelines

| Daily Log Volume | Server | ClickHouse |
|------------------|--------|------------|
| <1M logs | 2 cores, 2GB | 2 cores, 4GB |
| 1-10M logs | 4 cores, 4GB | 4 cores, 8GB |
| 10-100M logs | 8 cores, 8GB | 8 cores, 16GB |
| >100M logs | 16+ cores, 16GB+ | 16+ cores, 32GB+ |

### Disk Requirements

ClickHouse storage estimate:
- ~100-500 bytes per log entry (compressed)
- 1M logs/day ≈ 100-500 MB/day
- With 30-day retention ≈ 3-15 GB

---

## Monitoring

### Prometheus Metrics

BlazeLog exposes metrics at `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'blazelog'
    static_configs:
      - targets: ['blazelog-server:8080']
```

### Key Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `blazelog_logs_received_total` | Logs ingested | Rate drop |
| `blazelog_logs_processed_total` | Logs stored | < received |
| `blazelog_grpc_connections` | Active agents | < expected |
| `blazelog_buffer_size` | Buffered entries | > 80% capacity |
| `blazelog_clickhouse_insert_duration_seconds` | Insert latency | > 5s |

### Grafana Dashboard

Import the provided dashboard for visualization:

```bash
# Dashboard JSON in deployments/grafana/
curl -X POST \
  -H "Content-Type: application/json" \
  -d @deployments/grafana/blazelog-dashboard.json \
  http://grafana:3000/api/dashboards/db
```

---

## Common Bottlenecks

### High CPU Usage

**Agent:**
- Complex regex in parsers
- Too many file watchers
- **Fix:** Reduce sources, simplify patterns

**Server:**
- Too many concurrent connections
- Complex alert rules
- **Fix:** Increase resources, optimize rules

### High Memory Usage

**Agent:**
- Large batch_size
- Connection loss with high buffer
- **Fix:** Reduce batch_size, max_buffer_entries

**Server:**
- Many active SSE streams
- Large query results
- **Fix:** Add pagination limits, connection limits

### High Disk I/O

**ClickHouse:**
- Frequent small inserts
- Insufficient merge time
- **Fix:** Increase batch size, schedule merges off-peak

### Network Latency

**Agent → Server:**
- Small batch_size (too many RPCs)
- No compression
- **Fix:** Increase batch_size, enable compression

```yaml
# Enable gRPC compression
server:
  grpc:
    compression: "gzip"
```

---

## Benchmark Results

Tested on 4-core, 8GB server with ClickHouse:

| Scenario | Logs/Second | Latency (p99) |
|----------|-------------|---------------|
| Single agent, batch=100 | 5,000 | 50ms |
| Single agent, batch=500 | 15,000 | 100ms |
| 10 agents, batch=100 | 30,000 | 100ms |
| 10 agents, batch=500 | 80,000 | 200ms |

---

## See Also

- [Architecture Overview](ARCHITECTURE.md) - System design
- [Configuration Reference](CONFIGURATION.md) - All settings
- [Deployment Guide](DEPLOYMENT.md) - Installation
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Common issues

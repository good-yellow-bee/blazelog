# BlazeLog gRPC Protocol Specification

## Overview

BlazeLog uses gRPC for secure, efficient communication between agents and the central server. The protocol supports:

- Agent registration and configuration
- Bidirectional log streaming with acknowledgements
- Health monitoring via heartbeats
- Server-to-agent command delivery

## Connection Flow

```
Agent                                    Server
  |                                        |
  |-------- Register(AgentInfo) --------->|
  |<------- RegisterResponse -------------|
  |                                        |
  |======== StreamLogs (bidir) ===========>|
  |  LogBatch (seq=1) ------------------>  |
  |  LogBatch (seq=2) ------------------>  |
  |  <----------- StreamResponse (ack=2)   |
  |  LogBatch (seq=3) ------------------>  |
  |  ...                                   |
  |                                        |
  |-------- Heartbeat ------------------>  |
  |<------- HeartbeatResponse ------------|
  |                                        |
```

## Service Definition

```protobuf
service LogService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc StreamLogs(stream LogBatch) returns (stream StreamResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
}
```

## Message Types

### LogEntry

Core log data structure:

| Field | Type | Description |
|-------|------|-------------|
| timestamp | Timestamp | When event occurred |
| level | LogLevel | DEBUG/INFO/WARNING/ERROR/FATAL |
| message | string | Log message content |
| source | string | Log source identifier |
| type | LogType | NGINX/APACHE/MAGENTO/PRESTASHOP/WORDPRESS |
| raw | string | Original unparsed line |
| fields | Struct | Structured data (status code, request path, etc.) |
| labels | map | Key-value metadata |
| file_path | string | Source file path |
| line_number | int64 | Line number in source |

### LogBatch

Collection of LogEntries for efficient streaming:

| Field | Type | Description |
|-------|------|-------------|
| entries | repeated LogEntry | Array of log entries |
| agent_id | string | Sending agent ID |
| sequence | uint64 | Sequence number for ordering/dedup |

### AgentInfo

Agent metadata for registration:

| Field | Type | Description |
|-------|------|-------------|
| agent_id | string | Unique identifier |
| name | string | Human-readable name |
| hostname | string | Agent hostname |
| version | string | Agent version |
| os | string | Operating system |
| arch | string | CPU architecture |
| labels | map | Grouping/filtering metadata |
| sources | repeated LogSource | Configured log sources |

### StreamConfig

Server-provided configuration:

| Field | Type | Description |
|-------|------|-------------|
| max_batch_size | int32 | Maximum entries per batch |
| flush_interval_ms | int32 | Max time before flush |
| compression_enabled | bool | Enable compression |

### AgentStatus

Runtime status in heartbeats:

| Field | Type | Description |
|-------|------|-------------|
| entries_processed | uint64 | Entries since last heartbeat |
| buffer_size | uint64 | Currently buffered entries |
| active_sources | int32 | Active file watchers |
| errors | repeated string | Recent errors |
| memory_bytes | uint64 | Memory usage |
| cpu_percent | float | CPU usage percentage |

## Streaming Protocol

### Log Streaming (StreamLogs)

- **Bidirectional streaming RPC**
- Agent sends LogBatch messages continuously
- Server acknowledges with StreamResponse
- Server can inject commands via StreamResponse

### Batching Strategy

Default configuration (server can override via StreamConfig):

| Parameter | Default | Description |
|-----------|---------|-------------|
| max_batch_size | 100 | Maximum entries per batch |
| flush_interval_ms | 1000 | Maximum buffer time |
| compression_enabled | false | gRPC compression |

### Sequence Numbers

- Monotonically increasing per agent session
- Used for acknowledgement and deduplication
- Agent buffers unacked batches for retry on reconnect

## Heartbeat Protocol

| Parameter | Default | Description |
|-----------|---------|-------------|
| Interval | 30s | Time between heartbeats |
| Timeout | 10s | Response timeout |

Contains agent status metrics for monitoring.

## Server Commands

Commands sent via HeartbeatResponse or StreamResponse:

| Command | Description |
|---------|-------------|
| RELOAD_CONFIG | Reload agent configuration |
| PAUSE | Pause log streaming |
| RESUME | Resume log streaming |
| SHUTDOWN | Graceful agent shutdown |

## Error Handling

### Connection Loss

1. Agent detects disconnection
2. Buffers logs locally (up to configured limit)
3. Reconnects with exponential backoff
4. Resends unacknowledged batches after reconnection

### Invalid Messages

- Logged and skipped
- Connection maintained
- Error reported in next heartbeat

## Enums

### LogLevel

| Value | Number | Description |
|-------|--------|-------------|
| LOG_LEVEL_UNSPECIFIED | 0 | Unknown/unset |
| LOG_LEVEL_DEBUG | 1 | Debug messages |
| LOG_LEVEL_INFO | 2 | Informational |
| LOG_LEVEL_WARNING | 3 | Warnings |
| LOG_LEVEL_ERROR | 4 | Errors |
| LOG_LEVEL_FATAL | 5 | Fatal/critical |

### LogType

| Value | Number | Description |
|-------|--------|-------------|
| LOG_TYPE_UNSPECIFIED | 0 | Unknown/unset |
| LOG_TYPE_NGINX | 1 | Nginx logs |
| LOG_TYPE_APACHE | 2 | Apache logs |
| LOG_TYPE_MAGENTO | 3 | Magento logs |
| LOG_TYPE_PRESTASHOP | 4 | PrestaShop logs |
| LOG_TYPE_WORDPRESS | 5 | WordPress logs |

### Severity

| Value | Number | Description |
|-------|--------|-------------|
| SEVERITY_UNSPECIFIED | 0 | Unknown/unset |
| SEVERITY_LOW | 1 | Low priority |
| SEVERITY_MEDIUM | 2 | Medium priority |
| SEVERITY_HIGH | 3 | High priority |
| SEVERITY_CRITICAL | 4 | Critical priority |

### CommandType

| Value | Number | Description |
|-------|--------|-------------|
| COMMAND_TYPE_UNSPECIFIED | 0 | No command |
| COMMAND_TYPE_RELOAD_CONFIG | 1 | Reload config |
| COMMAND_TYPE_PAUSE | 2 | Pause streaming |
| COMMAND_TYPE_RESUME | 3 | Resume streaming |
| COMMAND_TYPE_SHUTDOWN | 4 | Shutdown agent |

## Security

mTLS will be added in Milestone 15:

- Mutual TLS authentication
- Certificate-based agent identity
- Encrypted transport
- Certificate rotation support

## Proto Files

Source proto files are located in `proto/blazelog/v1/`:

- `common.proto` - Shared enums
- `log.proto` - LogEntry, LogBatch
- `agent.proto` - Agent registration, heartbeat
- `alert.proto` - Alert messages
- `service.proto` - LogService definition

Generated Go code is in `internal/proto/blazelog/v1/`.

## Code Generation

```bash
# Install dependencies
make proto-deps

# Generate Go code
make proto

# Clean generated code
make proto-clean
```

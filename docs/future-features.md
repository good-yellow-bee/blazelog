# BlazeLog Future Features Roadmap

Inspired by Datadog Logs feature set.

## 1. Search & Discovery

| Feature | Description | Priority |
|---------|-------------|----------|
| Full-text search | Lucene-like queries across all log content | High |
| Faceted search | Click-to-filter on any field (level, source, status) | High |
| Saved views | User-saved queries/filters with naming | Medium |
| Search autocomplete | Field names, values, operators suggestions | Medium |
| Search history | Recent searches per user | Low |

## 2. Real-time Operations

| Feature | Description | Priority |
|---------|-------------|----------|
| Enhanced live tail | Real-time stream with complex filters, pause/resume | High |
| Context switching | View N logs before/after a specific log entry | High |
| Log sharing | Shareable URLs with preserved filters + time range | High |
| Keyboard shortcuts | j/k navigation, / for search, etc. | Medium |
| Log bookmarks | Mark important logs for later reference | Low |

## 3. Analytics

| Feature | Description | Priority |
|---------|-------------|----------|
| Log patterns | Auto-cluster similar logs, show frequency | High |
| Log-to-metrics | Generate time-series metrics from log fields | Medium |
| Anomaly detection | ML-based unusual pattern detection | Low |
| Top-N analysis | Most frequent errors, IPs, endpoints | Medium |
| Trend graphs | Volume over time by level/source | Medium |

## 4. Integrations

| Feature | Description | Priority |
|---------|-------------|----------|
| Webhook notifications | Generic HTTP POST for any system | High |
| PagerDuty | Incident management integration | Medium |
| OpsGenie | Alert routing and escalation | Medium |
| Jira | Auto-create tickets from alerts | Low |
| Discord | Notification channel | Low |

## 5. Storage & Retention

| Feature | Description | Priority |
|---------|-------------|----------|
| Retention policies | Per-project log TTL configuration | High |
| Log archives | Export to S3/GCS for cold storage | Medium |
| Archive rehydration | Query archived logs on-demand | Low |
| Log sampling | Reduce storage for high-volume sources | Medium |
| Compression settings | Configurable ClickHouse compression | Low |

---

## Implementation Status

- [ ] Phase 3: Real-time Operations (Next)
- [ ] Phase 4: Search & Discovery
- [ ] Phase 5: Analytics
- [ ] Phase 6: Integrations
- [ ] Phase 7: Storage & Retention

# Eventkit Event Schemas for Accounting

This document describes all eventkit event schemas for the accounting system, covering both instantaneous and time-aggregated usage events.

## Overview

The accounting system emits five types of events at key integration points:

1. **Storage Tally** - Instantaneous bucket storage snapshots
2. **Storage Rollup** - Time-aggregated storage usage (byte-hours per node)
3. **Order Settlement** - Bandwidth usage when storage nodes settle orders
4. **Inline Bandwidth Update** - Instantaneous inline bandwidth usage
5. **Bandwidth Rollup** - Time-aggregated bandwidth usage per node

## Event Type 1: `storage_tally` (Instantaneous)

**Location**: `satellite/accounting/tally/tally.go`
**Emission Point**: After bucket tallies are collected and saved to database
**Frequency**: Every tally interval (default: 1 hour)

### Purpose
Captures instantaneous snapshots of bucket storage metrics for real-time usage tracking and recalculation.

### Event Fields

| Field             | Type      | Description                                     | Example                     |
|-------------------|-----------|-------------------------------------------------|-----------------------------|
| `project_id`      | bytes     | UUID bytes of the project owning the bucket     | 16-byte UUID representation |
| `bucket_name`     | string    | Name of the bucket                              | `"my-bucket"`               |
| `tenant_id`       | string    | Tenant ID (nullable, reserved for future use)   | `""`                        |
| `placement`       | int64     | Placement constraint ID for the bucket          | `0`                         |
| `timestamp`       | timestamp | Time when the tally was collected               | `2025-01-15T10:30:00Z`      |
| `bytes`           | int64     | Total bytes stored in the bucket                | `1073741824`                |
| `segments`        | int64     | Total number of segments in the bucket          | `100`                       |
| `objects`         | int64     | Total number of committed objects in the bucket | `50`                        |
| `pending_objects` | int64     | Number of pending (uncommitted) objects         | `5`                         |
| `metadata_size`   | int64     | Total metadata size in bytes                    | `10240`                     |
| `event_type`      | string    | Always "instantaneous" for tally events         | `"instantaneous"`           |

### Configuration
```bash
--tally.eventkit-tracking-enabled=true
```

---

## Event Type 2: `storage_rollup` (Time-Aggregated)

**Location**: `satellite/accounting/rollup/rollup.go`
**Emission Point**: After storage tallies are rolled up and saved to database
**Frequency**: Every rollup interval

### Purpose
Tracks time-aggregated storage usage (data-at-rest) for accurate billing. Represents how much data was stored over time, measured in byte-hours.

### Event Fields

| Field            | Type      | Description                                                 | Example                                                 |
|------------------|-----------|-------------------------------------------------------------|---------------------------------------------------------|
| `node_id`        | string    | Storage node ID storing the data                            | `"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S"` |
| `tenant_id`      | string    | Tenant ID (reserved for future use)                         | `""`                                                    |
| `day`            | timestamp | Start of the day (midnight UTC)                             | `2025-01-15T00:00:00Z`                                  |
| `interval_start` | timestamp | Start of the rollup period                                  | `2025-01-15T00:00:00Z`                                  |
| `interval_end`   | timestamp | End of the rollup period                                    | `2025-01-15T23:59:59Z`                                  |
| `at_rest_total`  | float64   | Total data-at-rest in byte-hours (fractional for precision) | `8640000000000.5`                                       |
| `event_type`     | string    | Always "time_aggregated" for rollup events                  | `"time_aggregated"`                                     |

### Configuration
```bash
--rollup.eventkit-tracking-enabled=true
```

## Event Type 3: `order_settlement` (Instantaneous)

**Location**: `satellite/orders/endpoint.go`
**Emission Point**: After orders are successfully settled (storage node bandwidth recorded)
**Frequency**: Real-time as storage nodes settle orders

### Purpose
Tracks bandwidth usage when storage nodes settle orders, capturing both settled (actual used) and dead (allocated but unused) bandwidth.

### Event Fields

| Field           | Type      | Description                                   | Example                                                         |
|-----------------|-----------|-----------------------------------------------|-----------------------------------------------------------------|
| `node_id`       | bytes     | Storage node ID bytes settling the orders     | 32-byte node ID representation                                  |
| `project_id`    | bytes     | UUID bytes of the project                     | 16-byte UUID representation                                     |
| `bucket_name`   | string    | Name of the bucket                            | `"my-bucket"`                                                   |
| `tenant_id`     | string    | Tenant ID (nullable, reserved for future use) | `""`                                                            |
| `action`        | string    | Action type                                   | `"GET"`, `"PUT"`, `"GET_REPAIR"`, `"PUT_REPAIR"`, `"GET_AUDIT"` |
| `settled_bytes` | int64     | Actual bandwidth used (settled amount)        | `1048576`                                                       |
| `dead_bytes`    | int64     | Bandwidth allocated but not used              | `524288`                                                        |
| `window`        | timestamp | Settlement window timestamp                   | `2025-01-15T14:00:00Z`                                          |
| `timestamp`     | timestamp | Time when settlement was processed            | `2025-01-15T14:05:23Z`                                          |
| `event_type`    | string    | Always "instantaneous" for settlement events  | `"instantaneous"`                                               |

### Configuration
```bash
--orders.eventkit-tracking-enabled=true
```

## Event Type 4: `inline_bandwidth_update` (Instantaneous)

**Location**: `satellite/orders/service.go`
**Emission Point**: When inline bandwidth usage is recorded
**Frequency**: Real-time as inline data is accessed

### Purpose
Tracks bandwidth usage for inline segments. This complements the order_settlement events which track bandwidth through storage nodes.
This event may be emitted multiple times within an hour for the same bucket as inline data is accessed. Calculations should then
sum these instantaneous events over the hour to get total inline bandwidth usage.

### Event Fields

| Field            | Type      | Description                                        | Example                     |
|------------------|-----------|----------------------------------------------------|-----------------------------|
| `project_id`     | bytes     | UUID bytes of the project                          | 16-byte UUID representation |
| `bucket_name`    | string    | Name of the bucket                                 | `"my-bucket"`               |
| `tenant_id`      | string    | Tenant ID (nullable, reserved for future use)      | `""`                        |
| `bytes`          | int64     | Bandwidth used for inline data access              | `65536`                     |
| `interval_start` | timestamp | Start of the hourly interval                       | `2025-01-15T14:00:00Z`      |
| `interval_end`   | timestamp | End of the hourly interval                         | `2025-01-15T15:00:00Z`      |
| `timestamp`      | timestamp | Time when the bandwidth update occurred            | `2025-01-15T14:23:45Z`      |
| `event_type`     | string    | Always "instantaneous" for inline bandwidth events | `"instantaneous"`           |

### Configuration
```bash
--orders.eventkit-tracking-enabled=true
```

---

## Event Type 5: `bandwidth_rollup` (Time-Aggregated)

**Location**: `satellite/accounting/rollup/rollup.go`
**Emission Point**: After bandwidth is rolled up and saved to database
**Frequency**: Every rollup interval

### Purpose
Tracks time-aggregated bandwidth usage per node and action type for daily billing summaries.

### Event Fields

| Field              | Type      | Description                                | Example                        |
|--------------------|-----------|--------------------------------------------|--------------------------------|
| `node_id`          | bytes     | Storage node ID bytes                      | 32-byte node ID representation |
| `tenant_id`        | string    | Tenant ID (reserved for future use)        | `""`                           |
| `day`              | timestamp | Start of the day (midnight UTC)            | `2025-01-15T00:00:00Z`         |
| `put_total`        | int64     | Total PUT bandwidth (bytes)                | `10737418240`                  |
| `get_total`        | int64     | Total GET bandwidth (bytes)                | `21474836480`                  |
| `get_audit_total`  | int64     | Total GET_AUDIT bandwidth (bytes)          | `1073741824`                   |
| `get_repair_total` | int64     | Total GET_REPAIR bandwidth (bytes)         | `2147483648`                   |
| `put_repair_total` | int64     | Total PUT_REPAIR bandwidth (bytes)         | `1073741824`                   |
| `event_type`       | string    | Always "time_aggregated" for rollup events | `"time_aggregated"`            |

### Configuration
```bash
--rollup.eventkit-tracking-enabled=true
```

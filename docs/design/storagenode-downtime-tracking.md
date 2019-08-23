# Storage Node Downtime Tracking

## Abstract

The goal is tracking Storage Nodes downtime per Satellite.

## Background

The [Disqualification design document](disqualification.md) describes when the Storage Nodes get disqualified based on the reputation scores described in the [Node Selection design document](node-selection.md).

The current disqualification showed to us that several Storage Nodes got disqualified because of their uptime stats without giving clear and fair evidence of it to us nor their operators. We had to take action and invalidate the uptime's part of the formula used for calculating their reputation.

We need to enable back uptime disqualification, however before, we need to track in a fair way the amount of time Storage Nodes are offline.

This document presents a solution for tracking  Storage Node offline time, but it's out of the scope of this document how Storage Node reputation is affected by it.

The presented solution considers the requirements imposed by the [Kademlia removal design document](kademia-removal.md), where the Storage Nodes must contact the Satellite when they start and after, every hour.

## Design

For tracking Storage Nodes offline time we need:

- A new SQL database table to store offline time.
- Detecting which ones are offline.
- Roughly estimate how long they are offline.

An _uptime check_ referenced in this section is the connection initiated by the Satellite to any Storage Node in the same way that's described in [Network refreshing section of the Kademlia removal design document](kademia-removal.md#network-refreshing).

__NOTE__ the SQL code in this section is illustrative for explaining the algorithm concisely.

### Database

The following new SQL database table will store Storage Nodes offline time.

```sql
CREATE TABLE nodes_offline_time(
    node_id BYTEA NOT NULL,
    tracked_at timestamp with time zone NOT NULL,
    seconds integer,                                                    -- Measured number of seconds being offline
    CONSTRAINT nodes_offline_time__pk PRIMARY KEY(node_id, tracked_at)
)
```

The current Satellite database has the table `nodes`. For the offline time calculation we use the following columns:

- `last_contact_success`
- `last_contact_failure`
- `total_uptime_count`
- `uptime_success_count`

### Detecting offline nodes

An independent process runs, in a configurable interval, of time the following query:

```sql
SELECT
FROM nodes
WHERE
    last_contact_success < (now()  - 1h) AND
    last_contact_success > last_contact_failure AND
    disqualified IS NULL
ORDER BY
    last_contact_success ASC
LIMIT 1
```

The process runs the query repeatable until it doesn't return records then it ends and will run again in the next interval. The query only gets one record at the time for not holding a record set during the process.

For each node, the Satellite performs an _uptime check_.

* On success
    ```sql
        UPDATE nodes
        SET
            last_contact_success = now(),
            uptime_success_count = uptime_success_count + 1,
            total_uptime_count = total_uptime_count +1
        WHERE
            id = ?
    ```
* On failure:

  It calculates the number of offline seconds. We consider that the Storage Node must contact the Satellite every hour.

  Because we don't know exactly at what point it has got offline, we dismiss the hour between the last contact success and the supposed next one.

  ```
  num_seconds_offline =  seconds(from: last_contact_success, to: now() - 1h)
  ```

  ```sql
  INSERT INTO nodes_offline_time (node_id, tracked_time, seconds)
  VALUES (<<id>>, now(), <<num_seconds_offline>>)
  ```

  ```sql
  UPDATE nodes
  SET
    last_contact_failure = now(),
    total_uptime_count = total_uptime_count +1
  WHERE
    id = ?
   ```

### Estimating offline time

An independent process has the following configurable parameters:

- Sleep time.
- Number of nodes to check.

The process enters in a loop until it finishes, running the following query:

```sql
SELECT
FROM nodes
WHERE
    last_contact_success < last_contact_failure AND
    disqualified IS NULL
ORDER BY
    last_contact_failure ASC
LIMIT N
```

The query uses the number of nodes parameter for limiting the records.
It checks the configured number of nodes, then will sleep the configured amount of time and start again.

For each node performs an _uptime check_.

* On success:
  ```sql
  UPDATE nodes
  SET
    last_contact_success = now(),
    uptime_success_count = uptime_success_count + 1,
    total_uptime_count = total_uptime_count +1
  WHERE
    id = ?
  ```
* On failure:

  Calculate the number of seconds offline from now and the last contact failure.
  ```
  num_seconds_offline =  seconds(from: last_contact_failure, to: now())
  ```

  ```sql
  INSERT INTO nodes_offline_time (node_id, tracked_time, seconds)
  VALUES (<<id>>, now(), <<num_seconds_offline>>)
  ```

  ```sql
  UPDATE nodes
  SET
    last_contact_failure = now(),
    total_uptime_count = total_uptime_count +1
  WHERE
    id = ?
  ```

## Rationale

### Independent Service

The designed system runs in the Satellite single operative system process spinning up them in Goroutines. Currently, this is how all the different Satellite application processes run.

An alternate design is to create an independent service for running the process of the _offline downtime tracking_ described in the [design section](#design).

The advantages of this alternative are:

* It doesn't add a new application process to the Satellite.
* It's easier to scale.

And the disadvantages are:

* It requires to expose via a wire protocol the data selected from the nodes table. This cause more work and more latency apart from not offloading the current database<sup>1</sup>.
* It requires to update the deployment process.

The disadvantages outweigh the advantages of considering that:

* We want to start to track Storage Nodes offline time
* It doesn't offload the database despite being split in a different service.
* It may clash with the team working in the distributed Satellite architecture, or it would require to synchronize the tasks, hence more work and time to release.


<sup>1</sup> We want to reduce calls to the current database.

### InfluxDB

The designed system uses a SQL database for storing the Storage Nodes downtime.

An alternate solution is to use [InfluxDB time-series database](https://www.influxdata.com/).

The advantages are:

* Data Science team is already using it for data analysis.

And its disadvantages are:

* It requires to deploy InfluxDB for production, currently it's only used for collected metrics.

Unless the Data Science team manifest that InfluxDB is essential for them, this alternative doesn't outweigh the current design.


## Open issues

* The estimating offline time process will infinitely check permanent offline Storage Nodes unless they are marked as disqualified.

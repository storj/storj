# Storage Node Graceful Exit - User Interface

## Abstract

A Storage Node operator needs the ability to request a Graceful Exit per Satellite basis.
This document describes how the graceful exit interfaces with the operator.

## Background

The Storage Node operator needs:
- a way to initiate graceful exit,
- a way to monitor graceful exit progress, and
- terminate graceful exit without escrow.

## Design

Add a command `storagenode exit-satellite` to initiate a Graceful Exit.

The command should present a list of satellites to exit. The user needs to type the satellite domain name to start exiting. Note, remember to only list satellites that we haven't exited.

The satellite list should contain:
- domain name,
- node ID, and
- how much data is being stored.

Once the exit is initiated the command returns. The graceful exit process cannot be cancelled.

Initiating an graceful exit adds an entry with `satellite_id`, `initiated_at`, and `starting_disk_usage` to 
`graceful_exit_status` table. `starting_disk_usage` is loaded from `pieces.Service`. The graceful exit service starts a new worker for exiting, if one doesn't already exist.

TODO: how to show exit progress

TODO: how to terminate graceful exit?

## Rationale

We could use a design based on writing a number and then asking for confirmation. However, by requesting to type the domain name, it acts as a confirmation.

For `exit-satellite` command it could stay up and show exiting progress. However, exit could take several days and the storage node may even restart during the process.

## Implementation

- Add `graceful_exit_status` table and interfaces.
- Add `storagenode exit-satellite` command to storagenode CLI, which calls `gexit.Service.InitiateExit`.
	- Once initiated [protocol for transferring pieces](storagenode-graceful-exit-protocol.md) should start.
- TODO: monitoring exit progress
- TODO: terminating graceful exit?

Create `graceful_exit_status`
```
	model graceful_exit_status (
		key satellite_id

		field satellite_id              blob not null
		field initiated_at              timestamp ( autoinsert ) not null
		field completed_at              timestamp ( updateable )
		field starting_disk_usage       int64 not null
		field bytes_deleted             int64

		field completion_receipt  blob
	)
```

## Open issues (if applicable)

- Should we be able to terminate graceful exit? If we do not provide the feature, storage node operator might try to do this manually, breaking the whole node.
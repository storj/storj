# Storage Node Graceful Exit - User Interface

[Graceful Exit Overview](overview.md)

## Abstract

A storage node operator needs the ability to request a Graceful Exit on a per satellite basis.
This document describes how the storage node operator manages the Graceful Exit process.

## Background

The storage node operator needs:
- a way to initiate graceful exit, 
- a way to monitor graceful exit progress, and
- terminate graceful exit without escrow.

## Design

Add a command `storagenode exit-satellite` to initiate a Graceful Exit.

The command should present a message describing the consequences of starting a graceful exit. The user must confirm before continuing.

The command should present a list of satellites to exit. The user needs to type the satellite domain name to start exiting. Only satellites that haven't been exited will be displayed.

The satellite list should contain:
- Domain name
- Node ID
- How much data is being stored

Once the exit is initiated the command returns. The graceful exit process cannot be cancelled.

Initiating an graceful exit adds an entry with `satellite_id`, `initiated_at`, and `starting_disk_usage` to 
`satellites` table. `starting_disk_usage` is loaded from `pieces.Service`. The graceful exit service starts a new worker for exiting, if one doesn't already exist.

Add a command `storagenode exit-status` that a storage node operator can execute to get Graceful Exit status.  This report should return a list of exiting nodes with:
- Domain name
- Node ID
- Percent complete

TODO: how to terminate graceful exit?

## Rationale

We could use a design based on writing a number and then asking for confirmation. However, by requesting to type the domain name, it acts as a confirmation.

For `exit-satellite` command it could stay up and show exiting progress. However, exit could take several days and the storage node may even restart during the process.

## Implementation

- Add `storagenode exit-satellite` command to storagenode CLI, which updates `satellites` table with the satellite information that is being exited.
	- Once initiated [protocol for transferring pieces](protocol.md) should start.
- Add `storagenode exit-status` command to storagenode CLI. This returns completion status as described above.

## Open issues (if applicable)

- Should we be able to terminate graceful exit? If we do not provide the feature, storage node operator might try to do this manually, breaking the whole node. A solution to this should be considered in a future release.
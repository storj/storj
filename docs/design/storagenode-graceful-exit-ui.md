# Storage Node Graceful Exit - User Interface

## Abstract

A Storage Node operator needs the ability to request a Graceful Exit on a per Satellite basis.

## Background

The Storage Node operator needs a way to initiate a Graceful Exit without assistance from the Satellite operator. 

## Design

Provide storagenode CLI command to initiate a Graceful Exit. The command should present a list of Satellites to exit and a way to select an individual Satellite. On selection, the command should ask for confirmation before initiating the exit.

Once the exit is initiated, the exit process cannot be cancelled.

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation
- Add `gexit.exit_status` table
- Add `gracefulexit initiate` command to storagenode CLI
  - When executed, the user should be prompted with a numbered list of satellite IDs of whitelisted Satellites that have not been exited
  - After selecting a satellite, the user should be prompted for a confirmation
  - Once confirmed, the command should call the corresponding satellites `GracefulExit.Initiate` endpoint. See [Protocol for transferring pieces](storagenode-graceful-exit-protocol.md)
  - Creates new `gexit.exit_status` entry with `satellite_id`, `initiated_at`, and `starting_disk_usage` (based on a SUM of `pieceinfo_.piece_size` for the satellite being exited)
- Once initiated, the Graceful Exit Transfer service should start processing piece transfers. TODO: Details in new doc, or the protocol doc.

Create `exit_status`
```
	model exit_status (
		key satellite_id

		field satellite_id              blob not null
		field initiated_at              timestamp ( autoinsert ) not null
		field completed_at              timestamp ( updateable )
		field starting_disk_usage       int64 not null
		field bytes_deleted             int64
		field completed_exit_signature  blob
	)
```

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

# Storage Node Graceful Exit Overview

## Abstract

When a Storage Node wants to leave the network, we would like to ensure that the pieces get transferred to other nodes. This avoids additional repair cost and bandwidth. This design document describes the necessary components for the process.

## Background

Storage Nodes may want to leave the network. Taking a Storage Node directly offline would mean that the Satellite needs to repair the lost pieces. To avoid this we need a process to transfer the pieces to other nodes before taking the exiting node offline. We call this process Graceful Exit. Since this process can take time we need overview of the nodes exiting and the progress.

A Storage Node may also want to exit a single Satellite rather than the whole network. Example reasons could be that the Storage Node doesn't have as much bandwidth available or pricing **(TODO find better word)** for Satellites has changed.

The process must also consider that the Storage Nodes may have limited bandwidth available or patience. This means the process should prioritize pieces with smaller durability.

### Non-Goals

There are scenarios that this design document does not handle:

- Storage Node runs out of bandwidth during exiting.
- Storage Node may also want to transfer pieces partially due to decreased available capacity.
- Storage Node wants to rejoin a Satellite it previously exited.

## Design

The design is divided into four parts:

- [Process for gathering pieces that need to be transferred.](storagenode-graceful-exit-pieces.md)
- [Protocol for transferring pieces from one Storage Node to another.](storagenode-graceful-exit-protocol.md)
- [Reporting for graceful exit process.](storagenode-graceful-exit-reporting.md)
- [User Interface for interacting with graceful exit.](storagenode-graceful-exit-user-ui.md)

TODO: Constraints on how graceful exit happens.

TODO: Overview of the whole process.


## Open issues (if applicable)

- Ungraceful exit.
- Slow exit.
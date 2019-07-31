# Auditing V2: Random Node Selection

## Abstract

This design doc outlines how we will implement a Version 2 of the auditing service.
With our current auditing service, we are currently auditing per segment per data.
We need to keep this existing method of auditing, but also add an additional audit selection process that selects random nodes to audit.

## Background

As our network grows, it will take longer for nodes to get vetted.
This is because every time an upload happens, we send 5% of the uploaded data to unvetted nodes at 95% to vetted nodes.
When auditing occurs, we currently select a random stripe within a segment.
As more nodes join the network, it will become exponentially less likely that an unvetted node will be audited since most data will be stored on vetted nodes.

Currently, there's no way of selecting a stripe based on a storage node.

## Design

Two different loops will select audits:
- One loop will select based on bytes.
- The second loop will select based on nodes.

This way, both processes are calling the same audit path.

## Rationale

The chances of selecting the same stripe are rare, but should be accounted for.

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

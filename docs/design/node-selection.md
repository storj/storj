# Title: Reputation and Node Selection

## Abstract

Node selection is the process wherein the set of all possible storage nodes is reduced by the satellite for uploading segments.  Node selection applies to new file uploads via an uplink, as well as repair traffic from a satellite.  The node selection processes endeavors to fairly distribute upload traffic among storage nodes.  Node selection takes into consideration how new a node is, the overall performance characteristic of a storage node as characterized by its reputation score, and the IP address of each node.

## Background

The whitepaper describes the a 'preferences' system used in node selection, base on reputation:

> After disqualified storage nodes have been filtered out, remaining statistics collected during audits will be used to establish a preference for better storage nodes during uploads. These statistics include performance characteristics such as throughput and latency, history of reliability and uptime, geographic location, and other desirable qualities. They will be combined into a load-balancing selection process, such that all uploads are sent to qualified nodes, with a higher likelihood of uploads to preferred nodes, but with a non-zero chance for any qualified node.  Initially, we’ll be load balancing with these preferences via a randomized scheme, such as the Power of Two Choices, which selects two options entirely at random and then chooses the more qualified between those two. On the Storj network, preferential storage node reputation is only used to select where new data will be stored, both during repair and during the upload of new files, unlike disqualifying events.  If a storage node’s preferential reputation decreases, its file pieces will not be moved or repaired to other nodes.

The existing reputation-like system uses uptime and audit responses.  It does not currently consider geographic location, throughput, or latency.  In addition to factors which affect reputation, there are other considerations which are invovled in node selection.  These considersations currently include IP address, advertized available bandwidth, advertized available disk space, software version compatibility, and whether the node appeared to be online in the latest communication with the satellite.

## Design

[A precise statement of the design and its constituent subparts.]

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

# Deletion Performance

## Abstract

This document describes design for improvments to deletion speed.

## Background

Current design requires uplinks to send deletion information to all the storage nodes. This ends up taking a considerable amount of time.

There are few goals with regards to deletions:

- We need to ensure that uplinks are responsive.
- We need to ensure that storage nodes aren't storing too much garbage, since it reduces overall capacity of the network.
- We need to ensure that satellites aren't storing too much garbage, since that increases running cost of the satellite.

We have traced the uplink when removing files of different sizes and we obtained the following results:

<table style="border: 0.2rem solid black">
<thead style="border: 0.1rem solid black">
<tr style="border: 0.1rem solid black">
<td rowspan="3" style="border: 0.1rem solid black">File size</td>
<td rowspan="3" style="border: 0.1rem solid black">Inline</td>
<td colspan="6" style="width:69.56pt; " >Communication time in milliseconds</td>
</tr>
<tr style="border: 0.1rem solid black">
<td rowspan="2" style="border: 0.1rem solid black">Total</td>
<td colspan="4" style="border: 0.1rem solid black">Satellite</td>
<td rowspan="2" style="border: 0.1rem solid black">Storage Nodes</td>
</tr style="border: 0.1rem solid black">
<tr>
<td style="border: 0.1rem solid black">Dialing</td>
<td style="border: 0.1rem solid black">Information gathering (bucket, Object, …)</td>
<td>Mark beginning of deletion &amp; get list of segments</td>
<td style="border: 0.1rem solid black">Begin delete segment (delete segment metadata and return the list of order limits and private key of each piece)</td>
</tr>
</thead>
<tbody>
<tr style="border: 0.1em solid grey"><td style="border: 0.1em solid grey">1 Kib</td><td style="border: 0.1em solid grey">yes</td><td style="border: 0.1em solid grey">858</td><td style="border: 0.1em solid grey">277</td><td style="border: 0.1em solid grey">272</td><td style="border: 0.1em solid grey">144</td><td style="border: 0.1em solid grey">140</td><td style="border: 0.1em solid grey">0</td></tr><tr style="border: 0.1em solid grey"><td style="border: 0.1em solid grey">4 Kib</td><td style="border: 0.1em solid grey">yes</td><td style="border: 0.1em solid grey">910</td><td style="border: 0.1em solid grey">293</td><td style="border: 0.1em solid grey">278</td><td style="border: 0.1em solid grey">144</td><td style="border: 0.1em solid grey">142</td><td style="border: 0.1em solid grey">0</td></tr><tr style="border: 0.1em solid grey"><td style="border: 0.1em solid grey">5 Kib</td><td style="border: 0.1em solid grey">no</td><td style="border: 0.1em solid grey">1959</td><td style="border: 0.1em solid grey">328</td><td style="border: 0.1em solid grey">275</td><td style="border: 0.1em solid grey">142</td><td style="border: 0.1em solid grey">513</td><td style="border: 0.1em solid grey">652</td></tr><tr style="border: 0.1em solid grey"><td style="border: 0.1em solid grey">10 Kib</td><td style="border: 0.1em solid grey">no</td><td style="border: 0.1em solid grey">2451</td><td style="border: 0.1em solid grey">308</td><td style="border: 0.1em solid grey">278</td><td style="border: 0.1em solid grey">141</td><td style="border: 0.1em solid grey">560</td><td style="border: 0.1em solid grey">1134</td></tr><tr style="border: 0.1em solid grey"><td style="border: 0.1em solid grey">2.2 Mib</td><td style="border: 0.1em solid grey">no</td><td style="border: 0.1em solid grey">2643</td><td style="border: 0.1em solid grey">325</td><td style="border: 0.1em solid grey">285</td><td style="border: 0.1em solid grey">149</td><td style="border: 0.1em solid grey">560</td><td style="border: 0.1em solid grey">1273</td></tr><tr style="border: 0.1em solid grey"><td style="border: 0.1em solid grey">256 Mib</td><td style="border: 0.1em solid grey">no</td><td style="border: 0.1em solid grey">7591</td><td style="border: 0.1em solid grey">333</td><td style="border: 0.1em solid grey">275</td><td style="border: 0.1em solid grey">145</td><td style="border: 0.1em solid grey">1539</td><td style="border: 0.1em solid grey">6644</td></tr>
</tbody>
</table>

We extracted the data of the table the following trace graph files:

* [1kib](images/deletion-perfomance-1.0kb-file-trace.svg)
* [4Kib](images/deletion-perfomance-4.0kb-file-trace.svg)
* [5Kib](images/deletion-perfomance-5.0kb-file-trace.svg)
* [10kib](images/deletion-perfomance-10.0kb-file-trace.svg)
* [2.2Mib](images/deletion-perfomance-2.2mb-file-trace.svg)
* [256Mib](images/deletion-perfomance-256mb-file-trace.svg)


## Design

First, we can do reduce timeouts for delete requests. Undeleted pieces will eventually get garbage collected, so we can allow some of them to get lost.

The uplink should be able to issue the request without having to wait for the deletion to happen. Currently deletions are implemented as an RPC, instead, use a call without waiting for a response. For example, nodes could internally delete things async.

To reduce dialing time we should use UDP for delete requests.

Ensure we delete segments in parallel as much as possible.

All these actions will reduce the time that the uplink spends deleting the pieces from the storage nodes.

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

We could probablistically skip deleting pieces. This would minimize the requests that the uplink has to make, at the same time not leaking too much pieces. For example we could only send the deletion requests to only 50%, leaking half of the data, but making half the requests.

We could track how much each storage node is storing extra due not sending deletes. This would allow paying the storage nodes. However, this would still mean that garbage is being kept in the network.

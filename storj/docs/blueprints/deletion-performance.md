# Deletion Performance

## Abstract

This document describes design for improvements to deletion speed.

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

We extracted the data of the table the following trace graph _SVG_ files that you can find in a [Github gist](https://gist.github.com/ifraixedes/b178035b53161cb391b67026b70cba52).

## Design

Delegate to the satellite to make the delete requests to the storage nodes.

Uplink will send the delete request to the satellite and wait for its response.

The satellite will send the delete requests to all the storage nodes that have segments and delete the segments from the pointerDB. Finally, it will respond to the uplink.

The satellite will communicate with the storage nodes:

- Using the current network protocol, RPC/TCP.
- The request will be sent with long-tail cancellation in the same way as upload.
- It won't send requests to offline storage nodes.
- It will send requests concurrently as much as possible.

The satellite will use a backpressure mechanism for ensuring that it's responsive with uplink.
The garbage collection will delete the pieces in those storage nodes that didn't receive the delete requests.

The satellite could use a connectionless network protocol, like UDP, to send delete request to the storage nodes. We discard to introduce this change in the first implementation and consider it in the future if we struggle to accomplish the goals.

## Rationale

We have found some alternative approaches.
We list them, presenting their advantages and trade-offs regarding the designed approach.

### (1) Uplink delete pieces of the storage nodes more efficiently

As it currently does, uplink would delete the pieces from the storage nodes but with the following changes:

1. Reduce timeouts for delete requests.
1. Undeleted pieces will eventually get garbage collected, so we can allow some of them to get lost.
1. Uplink would send the request without waiting for the deletion to happen. For example, nodes could internally delete things async.
1. Send delete segments request concurrently as much as possible.

Additionally:

- We could change transport protocol for delete requests to a connectionless protocol, like UDP, for reducing dialing time.
- We could probabilistically skip deleting pieces to minimize the number of requests. For example, we could only send the deletion requests to only 50% of the pieces.

Advantages:

- No extra running and maintenance costs on the satellite side.

Disadvantages:

- Storage nodes will have more garbage than currently because of not waiting for storage nodes to confirm the operation.
- Storage nodes will have more garbage if we use a connectionless transport protocol
- Storage nodes will have more garbage if we use a probabilistic approach.


### (2) Satellite delete pieces of the storage nodes reliably

Uplink will communicate with the satellite as it currently does.

The satellite will take care of communicating with the storage nodes for deleting the pieces using RPC.

Advantages:

- Uplink deletion operation will be independent of the size of the file, guaranteeing always being responsive.
- It doesn't present a risk of leaving garbage in the storage nodes when deletion operation is interrupted.
- In general, the storage nodes will have less garbage because of deletions.

Disadvantages:

- The satellite requires a new chore to delete the pieces of the storage nodes. The increment of network traffic, computation, and data to track the segments to delete will increase the running costs.
- The satellite will have another component incrementing the cost of the operation as monitoring, replication, etc.


### (3) Satellite delete pieces of the storage nodes unreliably

Uplink will communicate with the satellite as it currently does.

The satellite will take care of communicating with the storage nodes for deleting the pieces using a connectionless protocol like UDP.

Advantages:

- Uplink deletion operation will be independent of the size of the file, guaranteeing always being responsive.
- It doesn't present a risk of leaving garbage in the storage nodes when deletion operation is interrupted.

Disadvantages:

- The satellite requires a new chore to delete the pieces of the storage nodes. The increment of network traffic, computation, and data to track the segments to delete will increase the running costs.
- The satellite will have another component incrementing the cost of the operation as monitoring, replication, etc.

### Conclusion

The alternative approach (1):

- It is similar to the current mechanism but with some improvements towards the goals.
- It doesn't add more load to the satellite, but we cannot trust in the uplink in deleting the pieces or informing the non-deleted ones.

The alternative approaches (2) and (3) are similar.

Approach (2) has the advantage of guaranteeing less garbage left on the storage nodes at the expense of requiring more network traffic.

Approach (3) requires less network traffic, but it may not require less computation considering that we may need to encrypt the data sent through UDP.

Approach (3) may get rid of the satellite garbage faster.

Both approaches, (2) and (3), present the problem of increasing garbage on the satellite side.

Taking one of these approaches will require a study on how to keep the less amount of garbage as possible.

## Implementation

### (1) Storage nodes:

1. Adapt protocol buffers definitions for delete operation.
       Satellite doesn't have to send order limits; it only has to send piece ID.
1. Create a new endpoint to receive delete requests from the satellite.

### (2) Satellite:

1. Implement delete request logic with backpressure mechanism to only confirm the operation when certain amount of storage nodes confirm successful.
   It's like what we do for upload long-tail cancellations.
1. Adapt protocol buffers definitions for delete operation.
   The current uplink RPC requests are:

        1. `BeginDeleteObject` - Retreives the _stream ID_.
        1. `ListSegments` - Uses _stream ID_  for retrieving the position index of all the segments.
        1. `BeginDeleteSegments` - Uses the _stream ID_ and position index for retrieving a list of _addressed order limits_.

   Because uplink won't send the delete requests to the storage nodes, the delete operation we can simplify it with one satellite request. The satellite will respond, with an empty body, when the deletion ends.

### (3) Uplink:

1. Change logic to not send delete requests to storage nodes.
1. Uplink `rm` command will wait until satellite responds.

### Considerations

If we plan to release the feature in several steps:

1. Implement and independently release (1) and (2) without removing the logic of the current functionality.
1. Implement and release (3).
1. Announce when previous versions will stop to work properly.
1. Remove and independently release the delete old logic from the storage node and satellite.


## Open issues (if applicable)

1. Should we track how much each storage node is storing extra due not sending deletes? For the storage nodes that accumulate too much garbage, we could send garbage collection request outside of the normal schedule.
1. Discuss backward-incompatibility that we may introduce when adapting the protocol buffer definitions in the storage node and satellite side. Should we reuse the current definition as much as possible although it isn't ideal?
1. How many storage nodes must confirm the deletion successful to allow the satellite to return a response to the uplink?

# Kademlia Removal

## Abstract

This design document outlines communication protocol between satellites and
storage nodes, network refreshes, and kademlia removal.

## Background

Many decentralized systems use Kademlia distributed hash table to find peers, exchange messages, and share data.
However, Storj only needs it for discovery and address lookups. 
Kademlia is useful when satellites don’t know about all the nodes in the network and nodes don't know the satellites.

To improve user experience, we decided to use an opt-in to satellites.
Opt-in means each storage node operator selects the satellites they want to work with.
As a result, storage nodes can notify satellites, without discovery.

The initial implementation has a hardcoded list of satellites in a configuration file.
Future improvements would add capabilities to manage the list dynamically.

As a result of all these decisions, we can remove Kademlia and replace it with direct communication.

## Design

To replace Kademlia, we need to complete several things:

- replace storage node initial communication with the satellites,
- replace network refreshing,
- remove kademlia from services,
- update documentation.


### Storage Node initial communication

Storage node should connect satellites in their trusted list and notify that they want to work with them.

We need to ensure that we do not overload the satellite during upgrades.
Hence, we need to add jitter for refreshes and initial communication.

_For more information on jitter see http://highscalability.com/blog/2012/4/17/youtube-strategy-adding-jitter-isnt-a-bug.html ._

### Network refreshing

Storage Nodes keep themselves up to date in the network by pinging all the satellites in their trusted list. Refreshes would happen every 1 hour.
Satellites, in response, will ping the nodes to confirm their address and ensure that the network is configured correctly.

When a Satellite has successfully pinged the storage node, it will update IP and uptime in overlay.
On failure, the satellite does not update overlay and notifies the storage node.

Storage Node keeps track of this information, such that Storage Node operator can notice the problem.

We consider a successful ping when a node with node-ID `N` has contacted satellite `S` and claimed its address is `A`,

1. `S` _must_ initiate a network connection `C` to address `A`
2. `S` _must_ verify that the remote endpoint on `C` has a private key corresponding to public key/node ID `N`. (It would be sufficient to complete negotiation of an SSL session over `C` and then verify that the remote end is using the same certificate used by `N` in the initial incoming ping.)
3. `S` _should_ verify that the remote endpoint on `C` agrees that its address is `A`. (This doesn't seem strictly necessary for security, but could prevent misconfigurations where a storage node operator runs multiple nodes with the same identity.)
4. `S` _should_ respond to the initial incoming ping from `N` with the result of the pingback, so that the dashboard on that node can report whether the node can receive incoming connections.


### Kademlia Removal

Once we have replaced the necessary pieces, we can remove Kademlia from the codebase:

- remove kademlia from discovery package,
- remove bootstrap node and server, and
- remove vouchers.

During all of these removals, we need to ensure that existing nodes do not break during upgrades.
It might be easier to replace endpoints with stubs, such that existing calls keep working until the network is fully upgraded.

### Update Documentation

We should update our documents with this major design change.
We need to update the whitepaper, audit gating design document, and wiki.

## Implementation

- [Nodes should communicate with satellites directly](https://storjlabs.atlassian.net/browse/V3-2274)
- [Network refreshes at a regular interval](https://storjlabs.atlassian.net/browse/V3-2275)
- [Remove the overlay cache from transport observers](https://storjlabs.atlassian.net/browse/V3-2305])
- [Delete Kademlia](https://storjlabs.atlassian.net/browse/V3-2276)
- [Update Documentation](https://storjlabs.atlassian.net/browse/V3-2461)

## Future considerations

### Satellite Management User Interface

Currently, Storage Node Operator can specify the list of satellites in the configuration.
Changing this configuration requires a restart and is not convenient.

Next steps would be to have a satellite management interface in the web-based console.
This means we need to store the satellite list in a database.

### NodeID updates

We should review whether we want to change NodeID or related data structures.
We may be able to simplify them further. As an example, it might be possible to remove NodeDossier.

### Retiring the Transport Observer

Transport observers currently update routing table and overlay during each connection.
Since routing table will be removed together with Kademlia, we can also simplify this design.

We can update uptime without using hooks, allowing to remove transport observer and hooks from the codebase.
It would also allow more clearly handle batching of uptime updates.

### Performant Storage Node and Satellite Pinging

We can reduce bandwidth and improve performance with UDP pinging.
Satellites can have a “tip box” to receive UDP messages about new nodes.
UDP messages need to be signed by the node and contain the address and the certificate chain.
Node ID-s with low difficulty are ignored.

Satellites regularly ping back the storage nodes and use the same protocol as described above.
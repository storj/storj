# Kademlia Removal

## Abstract

This design document outlines the communication protocol between satellites and
storage nodes, network refreshes, and kademlia removal given the satellite opt-in
capability for storage nodes.

## Background

Many peer-to-peer, decentralized systems employ the Kademlia implementation of a distributed hash table to allow for 
locating peer nodes, exchanging messages and sharing data. However, due to the nature of our network, we only use Kademlia 
for node discovery and address lookups given node IDs. This is useful when satellites don’t know about all the nodes in 
the network and nodes are unfamiliar with all of the satellites in the network.

With our recent business decision of simplifying the storage node operator user experience, we no longer require kademlia 
for node discovery. In a solution called SNO-select, storage nodes operators manually select the satellites they want to work with 
and satellites wait for storage nodes to work with them. The initial implementation of this solution allows SNOs to update 
their trusted satellite list in a hardcoded configuration file, but future improvements will enable users to manage
this list through a web console. 

We will replace our Kademlia DHT and related entities with direct communication between satellites and storage nodes, 
and keep the network fresh without kademlia node discovery and random lookups.

## Design

### Nodes reach out to satellites that they want to work with
- The satellites are listed in the trust package
- Nodes should communicate with satellites directly rather than using kademlia to traverse the network to find the address of a given ID.
- Storage nodes should notify satellites when they start up, wait a random amount of time (to add jitter 
http://highscalability.com/blog/2012/4/17/youtube-strategy-adding-jitter-isnt-a-bug.html), then start reporting in roughly on the hour

### Network refreshes at a regular interval
- Nodes will keep themselves up to date in the network by pinging all the satellites in their
   trusted list every hour.
- Satellites will ping the nodes back to confirm their addresses
    - If is it successful, the satellite will insert or update the node in the overlay cache and
       notify the node of success. Make sure to close the connection. Don’t use the transport observer to update the cache.
       Update the IP and uptime directly.
    - If the satellite does not confirm the node address, it does not proceed with updating the overlay cache. The node 
    receives a log message and closes the connection when it times out.

### Disintegrate Kademlia from the network, storj sim and testplanet setups
- Remove kademlia from the discovery package
- Remove the bootstrap node - work with Ops
  - Remove the vouchers service and related tables
  - Work with QA to make sure storage nodes don’t crash on errors related to the elimination of Kademlia
- if they don’t update immediately ->  keep just the overlay.Ping rpc method, it will be much easier for a new satellite 
to work with old and new storage nodes.

### Update whitepaper to address kademlia removal and the addition of satellite opt-in
  - Delete the audit gating design doc
  - Update the wiki

## Implementation

- [Nodes should communicate with satellites directly](https://storjlabs.atlassian.net/browse/V3-2274)

- [Network refreshes at a regular interval](https://storjlabs.atlassian.net/browse/V3-2275)

- [Remove the overlay cache from transport observers](https://storjlabs.atlassian.net/browse/V3-2305])

- [Delete Kademlia](https://storjlabs.atlassian.net/browse/V3-2276)

- [Update Documentation](https://storjlabs.atlassian.net/browse/V3-2461)

## Future considerations

### Selected satellite management
- Currently, a storage node operator can input a list of satellite IDs and addresses into their configuration file on setup. 
Several tardigrade-level satellites are included by default. 
- Next steps are to allow users to modify their selected satellites list through a web based console.
- The satellite list will need to be stored in a sql table or equivalent for persistence

### NodeID updates
- Is there anything that we should redesign regarding the nodeID and node data structures? 
- Do we need the node dossier any longer?

### Retiring the Transport Observer
- If the routing table and the overlay cache are the only features that use the transport observer, and we move to directly 
update the overlay cache, we can remove the transport observer. This would simplify uptime checks.

### Node -> satellite communication initiation
- To save resources and improve performance, satellites can have a “tip box” to receive UDP messages about new nodes
- UDP messages need the address of the node, the certificate chain to be expected once talking to the node that identifies 
the node, and a signature of the above things with the leaf private key of that certificate chain.
- Messages will ultimately be ignored if the difficulty of the computed ID isn't high enough or the node you end up talking 
to doesn't have the same node id as the one computed from the tipster certificate chain.

### Uptime checking
- Re-evaluate whether the data structures for keeping track of uptime are the right ones anymore
- Should satellites determine how often nodes are supposed to check in for uptime checks. Perhaps there is an 
"introduction ping" that occurs the first time the node comes online, and when the satellite responds, it includes how 
often nodes are expected to check back in to maintain good reputation
- [Uptime Disqualification Design Doc](https://github.com/storj/storj/pull/2733)


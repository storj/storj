# Kademlia Removal

## Abstract

This design document outlines the communication protocol between satellites and 
storage nodes, network refreshes, and kademlia removal given the satellite opt-in 
capability for storage nodes. 

## Background
User research on how Storage Node Operators would connect to satellites resulted in a 
simple solution called Opt-in SNO select. Satellites wait for storage nodes to work 
with them, and storage nodes manually select the satellites they want to work with. 
The initial implementation of this allows SNOs to update their trusted satellite list 
in a hardcoded configuration file, but future improvements will enable users to manage 
this list through a web console. Due to the nature of this change, the Storj network 
at this point in time no longer requires the Kademlia DHT or related entities. 
We will replace this protocol with direct communication between satellites and 
storage nodes, and keep the network fresh without kademlia node discovery and random 
lookups.

## Design

1. Nodes reach out to satellites that they want to work with
    
    - At the time of writing, a storage node operator can input a list of trusted satellite 
    IDs and addresses into their configuration file on setup. There are several tardigrade-level 
    satellites already listed as a preset. This list is imported into a Go map data structure 
    within the “trust” package.
    - Eventually during the design and implementation of trusted satellite management, 
    this configured list may be imported into a on-disk data store, eg SQL table. Users will 
    be able to modify this list through the storage node operator console. However, this is a 
    separate discussion from this design document. This design is satisfied with a static trusted 
    satellites list.
    - Nodes will be able to communicate with satellites through the “transport” and “trust” packages 
    directly rather than using kademlia to traverse the network to find the address of a given ID.

2. Network refreshes at a regular interval
    - Nodes will keep themselves up to date in the network by pinging all the satellites in their 
    trusted list every hour. 
    - Satellites will ping the nodes back
        - If is it successful, the satellite will insert or update the node in the overlay cache and 
        notify the node of success
        - If not, it does not proceed with updating the overlay cache, and the node receives an error message
    - This will use grpc for now, but future development may use a non-ssl connection from node to satellite 
    to initiate the refresh connection
    - Update the overlay cache refresh method to communicate with nodes directly, without relying on 
    kademlia lookups. Iterate through cache and dial each node.

3. Disintegrate Kademlia from the network, storj sim and testplanet setups
    - Remove the “discovery” package
    - Remove the bootstrap node - work with Ops
    - Work with QA to make sure storage nodes don’t crash on errors related to the elimination of Kademlia 
    if they don’t update immediately

4.Update whitepaper to address kademlia removal and the addition of satellite opt-in
    - Remove the audit gating design doc
    - Update the wiki

## Rationale
Many peer-to-peer, decentralized systems employ the Kademlia implementation of a distributed hash table to 
allow for locating peer nodes, exchanging messages and sharing data. However, due to the nature of our 
network, we only use Kademlia for node discovery and address lookups given node IDs. This is useful when 
satellites don’t know about all the nodes in the network and nodes are unfamiliar with all of the satellites 
in the network. But with our recent business decision of simplifying the storage node operator user experience, 
allowing nodes to directly select which satellites the would like to work with, we no longer require kademlia 
for node discovery.

## Implementation

1. [Nodes should find satellites through the trust packages rather than kademlia](https://storjlabs.atlassian.net/browse/V3-2274)

2. [Nodes keep up to date with satellites by periodically pinging and updating their trusted list](https://storjlabs.atlassian.net/browse/V3-2275)

3. [Remove kad random lookups for discovery & update overlay cache refresh](https://storjlabs.atlassian.net/browse/V3-2305])

4. [Disintegrate kademlia from the network, storj sim and testplanet setups](https://storjlabs.atlassian.net/browse/V3-2276)

5. Update Documentation

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]


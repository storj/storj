# Kademlia Removal

## Abstract

[A short summary of what this design doc accomplishes at a high level.]

satellite has up to date list of storagenodes from nodes talking to satellite
nodes can talk to satellites
both sides successfully connect to each other

every hour sn asks satellite to ping them so the are added/updated in the sat list

## Background
User research
Opt-in SNO select: satellite operators wait for storage nodes to select to work with them
storage nodes manually select teh satellites they want to work with
[An introduction of the necessary background and the problem being solved.]

## Design

1. Nodes keep up to date with satellites by periodically pinging and updating their trusted list

- Ping satellites in list every hour, update trusted list

- Satellite must pingback (secure tcp alternative to grpc connection? - future dev: non-ssl tip box)

    - if ok: satellite inserts/updates node in overlay cache and notifies node that they are successful

    - if not: don’t store in cache, and provide node with error message

2. Nodes should find satellites through the trust package rather than kademlia
    - SN should never use kad pkg

    - load from config into memory (done)
    - static unchanging list until *
    eg kad.FindNode replace with something like transport.DialNode(trust.Pool.GetAddress)
  
3. Remove kad random lookups for discovery & update overlay cache refresh
    - We no longer need the discovery package
    
    - Modify overlay cache to work directly with transport.DialNode

    - Iterate through overlay cache and dial all nodes
4. Disintegrate kademlia from the network, storj sim and testplanet setups
    - Make sure storage nodes don’t get crashing errors when they call kad methods (for 1 or two releases until they update)
    - test w Jens

5. Update whitepaper

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]

*future design around trusted satellite management

# Satellite Service Separation

## Abstract

The goal of the design doc is to describe the work necessary to make the Satellite horizontally scalable.

## Background

Currently all Satellite services run in a single binary in a single container. While this is great for development, this is bad for scaling.

The Satellite is made of up the following services:
- version
- overlay
- metainfo
- inspector
- orders
- repair (includes checker and repairer)
- audit
- garbage collection (GC)
- accounting (includes tally and rollup service and the live accounting cache)
- mail
- console
- marketing
- nodestats
- note: kademlia, discovery, and vouchers are being removed and not included in this doc

All services (except version) make connections to the masterDB. Five services rely on metainfo service to access the metainfoDB, this includes inspector, accounting, audit, repair, and garbage collection.

The current items that prevent Satellite horizontal scaling include:
- live accounting cache is currently stored in memory which cannot be shared across many replicas
- in-memory databases (i.e. boltDB and sqlite)
- ?

## Design

Break Satellite into many processes that can run independently in their own isolated environment (i.e. VM or container).  These isolated services should all be able to run replicas and load balance between them. So this means they need to share access to any persistent data.

## Rationale

WIP

## Implementation

WIP

## Open issues
- how will this affect development workflow
- how does this affect testing and storj-sim
- how does this affect deployments? 
- how to handle version of different services?
- what is the best order to do this work? in parallel maybe, but should we do certain services first?
- how does all this change with different dbs (i.e. spanner, etc)?
- is overlay cache cache happening?
- 

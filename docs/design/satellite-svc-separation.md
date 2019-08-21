# Satellite Service Separation

## Abstract

The goal of the design doc is to describe the work necessary to make the Satellite horizontally scalable.

## Background

Currently all Satellite services run in a single binary in a single container. While this is great for development, this is bad for scaling.

Currently the Satellite is a single binary made of up the following services:
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
- nodestats
- note: marketing service is currently on pause and is not included in this doc
- note: kademlia, discovery, and vouchers are being removed and not included in this doc

The Satellite has the following public GRPC endpoints:
- metainfo
- nodestats
- orders

The Satellite has the following privare GRPC endpoints:
- inspectors

All services (except version) make connections to the masterDB. Five services rely on metainfo service to access the metainfoDB, this includes inspectors, accounting, audit, repair, and garbage collection.

The current items that prevent Satellite horizontal scaling include:
- live accounting cache is currently stored in memory which cannot be shared across many replicas
- in-memory databases (i.e. boltDB and sqlite)
- potentially things that use mutexes, sync.Mutex or sync.RWMutex
- set on database, i.e. non transactional db changes
- ?

## Design

The plan here is to break the Satellite into many processes that can run independently in their own isolated environment (i.e. VM or container).  These isolated services should all be able to run replicas and load balance between them. So this means they need to share access to any persistent data.

Currently there is only one Satellite binary. We propose to add the following binaries:

1. api binary which includes the following: all public grpc endpoints, overlay, metainfo, nodestats
2. private api binary which includes: all private grpc endpoints, inspectors
3. console binary which includes: mail, console, overlay, metainfo
4. repair binary which includes: overlay, metainfo, orders
5. audit binary which includes: overlay, metainfo
6. accounting binary which includes: tally and rollup, overlay, metainfo
7. garbage collection binary which includes: overlay, metainfo
8. uptime binary which inludes: overlay
  - note: there is an ongoing discussion about the uptime service so this might change

The following diagram shows the above propsed design:

![Diagram of the above listed binaries](sa-separation-design.png)

## Rationale

WIP

## Implementation

Breaking apart each service will involve the following steps:
- create xProcess, where x is a Satellite service, i.e. repair, accounting, etc
- update satellite binary to include a subcommand so that the new xProcess can be run like this:
`satellite run repair`
- look for any areas in the code that prevent horizontal scaling for this new process and fix if found
- update testplanet so that unit tests still pass
- update storj-sim so that integration tests still pass
- create kubernetes deployment configs (this includes a dockerfile and an k8s HPA resource)
- automated deployment to staging kubernetes environment is setup and deploying

There is a prototype PR with an example that implements these steps (minus deployment configs) for the repair service, see that [here](https://github.com/storj/storj/pull/2836).

Other work:
- Add support for a cache for the live accounting cache so that its not stored in memory any longer.
- Update storj-sim to run with all the new services
- Update testplanet to run with all the new services

## Open issues
- how will this affect development workflow
- how does this affect testing and storj-sim
- how does this affect deployments? 
- how to handle version of different services?
- does any of this change with different dbs (i.e. spanner, etc)?
- is overlay cache cache happening?

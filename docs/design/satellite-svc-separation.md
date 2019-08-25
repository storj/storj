# Satellite Service Separation

## Abstract

The goal of the design doc is to describe the work necessary to make the Satellite horizontally scalable.

## Background

Currently all Satellite services run in a single binary in a single container. While this is great for development, this is bad for scaling.

### Current Satellite design

Currently the Satellite is a single binary made of up the following services:

#### overlay
wip

#### metainfo
Metainfo is responsible for all things related to the metainfo stored for each file on the network. The metainfo system is currently composed of the following parts: 1) metainfoDB, a key/value store with segment path names as the key and a pointer/segment object as the value, 2) metainfo loop which iterates over all the key/values in metainfDB, 3) metainfo endpoint which creates the public GRPCs for creating/deleting metainfo, 4) metainfo service.

#### orders
Orders is responsible for creating/managing orders that the satellite issues for a file upload/download. Orders are used to keep track of how much bandwidth was used to upload/download a file. This data is used to pay storage nodes for bandwidth usage and to charge the uplinks. See this [doc on the lifecycle of data](https://github.com/storj/docs/blob/master/code/payments/Accounting.md#lifecycle-of-the-data) related to accounting which includes orders.

#### audit
Audit performs audits of the storage nodes to make sure the data they store is still retrievable. The audit system is currently made up of an audit service that runs on an interval performing audits on a segment at a time. The result of the audits are reported to the overlay service to store in node table in Satellite.DB. See [docs on audit](https://github.com/storj/docs/blob/master/code/audits/audit-service.md) for more details.

#### repair
Repair loops over the metainfoDB and checks if a pointer is injured, if it is injured, its added to the repair queue and a repairer later fixes it. If its unable to be repaired then its added to irreparabledb. The irreparable loop later iterates over this table and attempts to re-repair these injured segments. The repair system is currently made of 4 parts and 2 DBs (db tables). The 4 parts are 1) repair observer (contains ReliabilityCache) 2) irreparable loop 3) repairer 4) repair queue. The 2 DBs are 1) injuredsegment (repair queue) table in satellite.DB 2) and irreparabledb table in satellite.DB 

#### garbage collection (GC)
GC consults with the metainfoDB, the source of truth for what data should be on the network, then with this info creates lists of what data each storage node should be storing and sends those lists to their respective storgae node.  The storage nodes can then delete any data that they are storing that is no longer in the GC list. See [GC design doc](https://github.com/storj/storj/blob/master/docs/design/garbage-collection.md) for more details. 

#### accounting
Accounting includes tally and rollup service along with the live accounting cache. The function of accounting is to calculate what to charge Uplinks for their usage and also what to pay storage nodes for their contributions to the network. See [docs on accounting](https://github.com/storj/docs/blob/master/code/payments/Accounting.md) for more details.

#### console
Console provides the web UI for the Satellite where users can create new accounts/projects/apiKeys needed for uploading/downloading to the network.

#### mail
Mail sends email for the console UI.

#### nodestats
Nodestats makes it so that storagenodes can ask the satellite for info about itself, for example, it can ask for stats on reputation and accounting storage usage.

#### version
Version publishes what version of the satellite are supported.

#### inspectors
Inspectors are exposed on a private GRPC port and provide a way to get data about certain systems. The following inspectors currently exist: overlay inspector, health inspector, and irreparable inspector.

#### kademlia
Kademlia, discovery, bootstrap, and vouchers are being removed and not included in this doc. See [kademlia removal design doc](https://github.com/storj/storj/blob/master/docs/design/kademlia-removal.md) for more details.

#### GPRC endpoints
The Satellite has the following GRPC endpoints:
- Public: metainfo, nodestats, orders, overlay (currently part of kademlia, but may be added here)
- Private: inspectors

#### HTTP endpoints
The Satellite has the following HTTP endpoints:
- Public: console
- Private: marketing admin UI, debug

#### databases
All services (except version) make connections to the masterDB. Five services rely on metainfo service to access the metainfoDB, this includes inspectors, accounting, audit, repair, and garbage collection.

See these docs for details on current database design: 
- https://github.com/storj/docs/blob/master/code/Database.md#database
- https://github.com/storj/docs/blob/master/code/persistentstorage/Databases.md

#### limitations
The current items that prevent Satellite horizontal scaling include:
- live accounting cache is currently stored in memory which cannot be shared across many replicas
- in-memory/locally stored databases. This includes revocationDB and the devDefaults for satellite.DB and metainfoDB.
- potentially things that use mutexes, sync.Mutex or sync.RWMutex (these don't prevent, but indicate that there might be shared state)
- set on database, i.e. non transactional db changes
- ?

## Design

The plan is to break the Satellite into multiple processes. Each process runs independently in their own isolated environment. Ideally, each process can be replicated and load balanced. This means they need to share access to any persistent data.

### New satellite binaries

Currently there is only one Satellite binary. We propose to add the following binaries:

#### satellite api
The satellite api will handle all public GRPC and HTTP requests, this includes all public endpoints for nodestats, overlay, orders, metainfo, and console web UI. It will need all the code to successfully process these public requests, but no more than that. If the console needs the mail service then that can be added as well, but make sure to only include the specific parts of the mail service it needs and don't include any background jobs if there are any.

#### private api
The private api binary handles all private GRPC and HTTP requests, this includes inspectors (overlay, health, irreparable), debug endpoints, and the marketing web UI. Open question: do we need the inspectors, if not should they be removed?

#### metainfo loop and the observer system
The metainfo loop binary iterates over all the pointers in metainfoDB repeatedly on an interval. With each loop, the process can also execute the code for the observer systems that take a pointer as input and performs some action with it. The observer systems currently include: audit observer, gc observer, repair checker observer, and accounting tally.

The audit observer uses the pointers from the metainfo loop to create segment reservoir samples for each storage node and saves those samples to a reservoir cache. Audit observer currently runs on a 30s interval for the release default setting. See [audit-v2 design](https://github.com/storj/storj/blob/master/docs/design/audit-v2.md) for more details.

The repair (checker) observer uses the pointers from the metainfo loop to identify segments that need to be repaired and adds those injured segments to the repair queue. The repair check currently has a checkerObserver.ReliabilityCache, is this cache ok to stay in-memory? The repair observer currently runs on a 30s interval for the release default setting.

The garbage collector (GC) observer uses the pointers from the metainfo loop to create bloom filters for each storge node. The bloom filters contain all the pieceIDs that the storage node should have. The bloom filters are stored in-memory then sent to the appropriate storage node. Keeping the bloom filters in-memory is ok as long as we don't run more replicas of the metainfo loop service, is this what we want? The GC observer currently runs on 5 day interval for the release default setting.

The tally observer uses the pointers from the metainfo loop to sum the total data stored on storage nodes and in buckets then saves these values in the tables `storagenode_storage_tally` and `bucket_storage_tally` in the Satellite.DB database.

The following diagram outlines the metainfo loop with the 4 observer:

<!-- ![Diagram of the above described metainfo loop observers](metainfo-loop-observers.png) -->
![Diagram of the above described metainfo loop observers](metainfo-loop-design.svg)

#### irreparable loop
The irreparable loop iterates through the irreparabledb table in Satellite.DB and attempts to repair the segment by adding to the reapir queue again. 

#### repair workers
The repair worker execute a repair for an item in the repair queue. We want to work through the repair queue as fast as possible so its important to be able to dispatch many workers at a time.

#### audit workers
The audit binary should be able to run many audits in parallel. For the [audit-v2 design](https://github.com/storj/storj/blob/master/docs/design/audit-v2.md) there is a reservoir cache that will need to be shared between the audit observer that loops through the metainfoDB and the audit process which executes the audits on a segment from the reservoir cache. It might be worthwhile to have two tables in the reservoir cache, one for vetted and one for unvetted. Then we can run two different audit processes, one for vetted and one for unvetted. The only difference between the vetted and unvetted audit processes would be 1) which cache table they read from and 2) how frequently they audit. This separation might be nice so that it's easier to keep track of and modify how many vetted vs unvetted audits are occuring.

The following diagram outlines the design for the audit system once separated out of the satellite:

![Diagram of the audit v2 system](audit-v2-design.png)

#### accounting
The accounting binary is responsible for calculating invoices for uplinks and payments for storage nodes. In order to do this, accounting must track total amounts of disk usage and bandwidth used by storage nodes and by buckets. Accounting should receive storage node total stored bytes data from the tally observer running with the metainfo loop.

#### version
Can we remove the version service? Or do we want to keep it and let these separated binaries check in with the version service to check they are running an appropriate version? Not sure if that is necessary since we could handle version compatibility at the layer of infrastructure deployment.

#### uptime ?
Is uptime going to be a system?

*** 

The following diagram shows the above propsed design:

![Diagram of the above listed binaries](sa-separation-design.png)

*** 

## Rationale

#### metainfo loop and observer system
For database performance reasons we should only have one thing looping over the metainfo database. This is why we are combining everything into the metainfo loop observer system. There is no sense in having multiple different processes all redoing the work of iterating over the database and parsing protobufs if lots of things need to do that same work over and over. Even with the observers running on different intervals, we still thinks its fine to combine all the 

There has been discussion about making the audit and GC observers run on differnt loops, but for performance concerns, its been decided to run all observers on the same metainfo loop (though GC will run less frequently). 

An additional is that for each metainfoDB iteration, we might want to choose a random starting position
so that the reservoir samples for the audit observer aren't biased toward the items at the beginning of the metainfoDB. If we start at a random location we can still loop through everything, we just have to make sure to wrap back to the beginning when we reach the end and keep going until we get back to the starting place. Keep in mind it's important for the GC observer that we hit every single item in the metainfoDB otherwise we could mess up GC output if we don't get an accurate list of current pieceIDs for storage nodes.

For the metainfo loop and observer system its not critical to have high availability for these systems, therefore its ok to have all these observers depend on a single metainfo loop process.  In the worst case scenario, if the metainfo loop goes down there may be downtime until its fixed and back up, this should be fine for small periods of time.

#### satellite api

We chose to combine all public GRPC and HTTP endpoints into one binary. The reason for this is so that we handle all requests in one place so we can scale handling requests up easier.

## Implementation

Note: getting Kademlia removed is a blocker to implementation.

We should break out one process at a time from the Satellite.  Here are the things we should do first since they impedes scale the most:
- satellite api
- repair workers
- audit workers

We will need to add a `SatelliteSystem` in `testplanet` so that we can test continue using the same unit tests for the Satellite as we break it apart. See an example of that [here](https://github.com/storj/storj/pull/2836/files#diff-c2ce7a2c9b2f4920e2de524c0fffd2f1R70).

For each new satellite binary we need to do the following steps:
- create xProcess, where x is a Satellite service, e.g. RepairProcess, AccountingProcess, etc
- update satellite binary to include a subcommand so that the new xProcess can be run like this:
`satellite run repair`
- look for any areas in the code that prevent horizontal scaling for this new process and fix if found
- update testplanet so that unit tests still pass
- update storj-sim so that integration tests still pass
- create kubernetes deployment configs (this includes a dockerfile and an k8s HPA resource)
- automated deployment to staging kubernetes environment is setup and deploying

There is a prototype PR with an example that implements these steps (minus deployment configs) for the repair service, see that [here](https://github.com/storj/storj/pull/2836). This is now out of date since we are adding the repair (checker) observer to the metainfo loop observer system, but this prototype is still useful as an example.

Other work:
- Add support for a cache for the live accounting cache so that its not stored in memory any longer.
- add support for postgres to revocationDB.
- Update storj-sim to run with all the new services.

## Open issues
- with kademlia/discovery/bootstrap being removed, what will be taking the place of the functionality still needed? For example, what will handle the storage nodes pinging satellite to ensure uptime checks?
- is uptime system going to be a thing?
- will we need to run more than one replica of the metainfo loop and its observer system? if so, there are two areas that may prevent scaling: 1) the repair.checkerObserver.ReliabilityCache and 2) the GC observer storing bloom filters in-memory.
- Do we need to keep the inspectors? Does anyone use those? If no, can we remove?
- how do we update storj-sim with these changes?
- how to handle version of different services? do we want to keep version service around?
- does any of this change with different dbs (i.e. spanner, etc)?

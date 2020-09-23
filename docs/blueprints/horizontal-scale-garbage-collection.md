# Horizontally Scaling Garbage Collection

## Abstract

The goal of this design doc is to describe a way to horizontally scale the garbage collection (GC) process. Currently GC is a single process with heavy memory use. Eventually GC will run out of memory or have too much data for a single machine so we need to be able to horizontally scale the garbage collection workloads.

## Background

The garbage collection process informs storage nodes if there are files they need to delete. It does this by making an in-memory list of all the files the satellite knows about for each storage node. An in-memory bloom filter data structure is used to store the list of files on the Satellite. See original garbage collection [design doc](garbage-collection.md) for more details.

Currently the largest Satellite stores 3.5 PB of data, with a total piece count of just over 4 billion, on over 12k storage nodes. When GC runs, this results in an approximate 2.5 GB total bloom filter size. The bloom filter size is relative to the node count and piece count. This means as the data and node count grows on the network so does the bloom filter size.

#### Estimate growth of bloom filter sizes:

Example for SLC Satellite:

current piece count: 4,214,619,943

total data currently stored: 3.2PB

total current bloom filter size: ~2.5 GB

If we have 10x the data, so ~30PB, then this will be 25 GB bloom filter size (it scales linearly).

If we store 1 EB of data, then there will be about ~1TB of bloom filter size.

Reference: [Redash query](https://redash.datasci.storj.io/queries/1224) for current GC sizes for each Satellite.

## Design

#### GC Manager and Workers
The idea here is to split the garbage collection process into a manager process and many worker processes. The GC Manager will join the metainfo loop to retrieve pointer data. It will assign a portion of the storage nodes to each of the GC workers so that each worker is only responsible for creating bloom filters for a subset of all storage nodes. The GC master will send the piece IDs from the metainfo loop to the correct worker responsible for that storage node. Once the GC cycle is complete, the workers send the completed bloom filters to the storage nodes.

#### Reliable data transfer mechanism
With this design, the GC Manager will be sending piece ID data to the workers, it's very important we never lose a piece ID. We need a way to confirm each worker received every piece ID from the GC Manager for any given GC iteration. Otherwise it could cause the storage node to delete the unintended data. One way to solve this is to assign a sequence number to each piece ID sent from GC Manager to the worker so that at the end of the GC cycle, the manager and worker must confirm the end sequence number match and all sequences in between have been received.

#### GC Sessions
Since the GC process runs on an interval, the GC workers need to keep track of which GC cycle relates to which bloom filters, we should store some sort of session ID or timestamp related to each GC cycle to track this.

## Implementation

GC Manager responsibilities:
- joins metainfo loop at the desired GC interval (currently 5 days)
- for each metainfo loop create a new session specific to that GC cycle, this will be used to keep track of which GC cycle the data corresponds to. The session might just be the timestamp the loop began.
- keep track of the total piece count processed for each storage node
- for each GC worker, the GC manager sends an RPC to indicate a new GC cycle is beginning
- process each pointer from the metainfo loop and send each piece id to the correct GC worker
- end the session once the metainfo loop is complete
- send a RPC to each GC worker with the total pieces they should have processed for each storage node to confirm all are accounted for

GC Worker responsibilities:
- receive piece ids from the GC manager
- create bloom filters for a subset of the storage nodes with the piece ids
- send completed bloom filter to the appropriate storage node

The storage node ID space will need to be partitioned based on how many GC workers there are, so that it's relatively evenly spread out. The GC Manager will partition the storage node ID space right before every GC iteration. The GC Manager will ping all workers before every GC iteration, the healthy workers will be active in the next GC cycle. The GC Manager will keep a map in memory of GC worker addresses to the start/end storage node ID it's responsible for.

If a GC worker fails during the GC cycle, for simplicity, for now we will just wait til the next GC cycle to catch up those missed storage nodes.

#### RPCs

RPCs between the GC Manager and the GC Workers for when each GC cycle runs:
```
service GarbageCollection {
    // The GC Manager will ping the workers before each GC cycle to make sure they are available
    // to participate in next cycle
    rpc Ping(PingRequest) returns (PingResponse) {}
    rpc StartSession(StartSessionRequest) returns (StartSessionResponse) {}
    rpc AddPiece(AddPieceRequest) returns (AddPieceResponse) {}
    rpc EndSession(EndSessionRequest) returns (EndSessionResponse) {}
}

message StartSessionRequest {
    google.protobuf.Timestamp session_id = 1 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}
message StartSessionResponse {
}

message AddPieceRequest {
    // session_id indicates which GC session this piece ID belongs to
    google.protobuf.Timestamp session_id = 1 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    // piece is the piece_id that should be added to the bloom filter
    bytes piece_id = 2 [(gogoproto.customtype) = "PieceID", (gogoproto.nullable) = false];}

    // sequence_number is the ordered number assigned to this request so the worker can confirm they received all the correct pieces
    int64  sequence_number = 3;

    // storage node_id is the id of the storage node this piece is stored on
    bytes storage node_id = 4 [(gogoproto.customtype) = "NodeID", (gogoproto.nullable) = false];
}
message AddPieceResponse {
}

message EndSessionRequest {
    // session_id indicates which session to end
    google.protobuf.Timestamp session_id = 1 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    // node_ending_sequence is a map of storage node ID to its corresponding ending sequence number
    map<bytes, int64> node_ending_sequence = 2;
}
message EndSessionResponse {
}
```

## Rationale

Alternatives considered:
1. Write bloom filters to disk. Cons: very slow, also we try to avoid storing persistent data in kubernetes.

2. Store bloom filters in a remote cache or remote data store. We currently have a redis instance that the Satellite uses, but it is only 1gb size so if we wanted to explore this option, we would need to greatly increase the size.

3. Make bloom filter size smaller by altering the falsePositiveRate or hard code the size of the bloom filters instead of basing it off of piece count.The bloom filter size is related to node count and piece count, but we could hard code the bloom filter size so that it only relates to node count. However we need to better understand the false positive impact of this before moving forward.

4. Only process a fraction of the storage nodes at one time. Currently GC runs every 5 days so instead of running once every 5 days, instead run continuously.
- Cons:
  - This will hit a limit scaling once it takes longer than 5 days to process all of GC.
  - GC runs on metainfo loop, so this will increase load on miLoop and also how long it takes for GC to run will be dependent on how long the miLoop takes to run.
  - If we decide in the future we want to run GC more frequently that would impact this.

5. Each time the GC runs, create or populate a database table that has 2 columns: storage node ID and piece ID. Then when GC joins the metainfoLoop, it would fill out this table and add a new row for each storage nodeID to the pieceID pair. Once the metainfo loop is complete and the table is done, then iterate over the table to create a bloom filter for one storage node at a time. Once all bloom filters are created, the data in the new table can be deleted so that it doesn't get stale over time.
- Cons:
  - storing duplicate data in different places is bug prone
  - this temporary table might be very big. For example, for SLC Satellite it would currently be ~130 GB
  - it sounds like the metainfoDB might need to be a relational database instead of creating 2 different tables with duplicate data.

## Open issues

1. How should we handle scaling the GC workers. The options seem to be:
a) hard code a list of GC workers addresses in the GC Manager config
b) create a RPC the new GC workers can register with the GC Manager to join the next GC cycle. The GC Manager would need to re-partition the storage node ID space each time a new worker joins.

2. If a GC Worker does not receive one of the sequences, should it reply with a NAK and request the missing sequence number? Or should it just abort this cycle for that storage node and try again next GC cycle?

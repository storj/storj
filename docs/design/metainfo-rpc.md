# Metainfo RPC Service Refactor

## Background

Our current metainfo rpc service isn't very future proof.
If we shipped our current client code, it wouldn't be easy to update clients and add support for the following:
- versions
- multipart uploads
- cleanup for uncommitted segments
- general concurrency fixes

Native multipart upload can't happen with our current architecture because there are issues with out of order uploading of a single stream.

Currently, concurrent uploading causes segments to get mangled, but adding versions is a good way to handle concurrent uploads.
The current protobuf does not contain which version is being written, so it has to assume "version 0", which is problematic with concurrent clients (they both are writing to the "version 0").

This should also reduce the number of roundtrips needed to start and end both streams and segments.

This design should ensure compatibility between client and server in the future as these features are implemented. Neither clients nor servers will be able to access more information than what we provide in this new proto, so we need to make sure the messages and service calls anticipate needs for the above features.

## Design

The following is a proto file for just the rpc portion to create segments. The read portions just need to have a stream id included, which should default to being the largest stream id.
(When we implement versions, every upload will have a new stream, which will be given the largest stream id at that time. This will allow us to get older versions by checking the stream id.)

With this design, streams will be created that contain multiple parts. There were already RPCs to finalize individual segments, but each segment was not logically associated with a part, but instead just the specific object.

```protobuf
message AddressedOrderLimit {
    orders.OrderLimit2 limit = 1;
    node.NodeAddress storage_node_address = 2;
}

service Metainfo {
    rpc StartStreamAndStartSegment(StartStreamAndStartSegmentRequest) returns (StartStreamAndStartSegmentResponse);
    rpc CommitSegmentAndStartSegment(CommitSegmentAndStartSegmentRequest) returns (CommitSegmentAndStartSegmentResponse);
    rpc CommitSegmentAndCommitStream(CommitSegmentAndCommitStreamRequest) returns (CommitSegmentAndCommitStreamResponse);

    // reads / deletes
}

message StartStreamAndStartSegmentRequest {
    // Start Stream
    bytes bucket = 1;
    bytes path = 2;
    int64 stream_id = 3; // version, etc. -1 for server to pick.

    // Start Segment
    int64 segment_index = 4;
    int64 part_number = 5;

    // the following copied from an earlier version of these requests
    pointerdb.RedundancyScheme redundancy = 6;
    int64 max_encrypted_segment_size = 7;
    google.protobuf.Timestamp expiration = 8;
}

message CommitSegmentAndStartSegmentRequest {
    // Commit Segment
    bytes segment_upload_id = 1;
    pointerdb.Pointer pointer = 2;
    repeated orders.OrderLimit2 original_limits = 3;

    // Start Segment
    bytes stream_upload_id = 4;
    int64 segment_index = 5;
    int64 part_number = 6;

    // The following copied from an earlier version of these requests
    pointerdb.RedundancyScheme redundancy = 7;
    int64 max_encrypted_segment_size = 8;
    google.protobuf.Timestamp expiration = 9;
}

message CommitSegmentAndCommitStreamRequest {
    // Commit Segment
    bytes segment_upload_id = 1;
    pointerdb.Pointer pointer = 2;
    repeated orders.OrderLimit2 original_limits = 3;

    // Commit Stream
    bytes stream_upload_id = 4;
}

message StartStreamAndStartSegmentResponse {
    // Start Stream
    bytes stream_upload_id = 1;

    // Start Segment
    bytes segment_upload_id = 2;
    repeated AddressedOrderLimit addressed_limits = 3;
    bytes root_piece_id = 4 [(gogoproto.customtype) = "PieceID", (gogoproto.nullable) = false];
}

message CommitSegmentAndStartSegmentResponse {
    // Commit Segment
    pointerdb.Pointer pointer = 1;

    // Start Segment
    bytes segment_upload_id = 2;
    repeated AddressedOrderLimit addressed_limits = 3;
    bytes root_piece_id = 4 [(gogoproto.customtype) = "PieceID", (gogoproto.nullable) = false];
}

message CommitSegmentAndCommitStreamResponse {
    // Commit Segment
    pointerdb.Pointer pointer = 1;

    // Commit Stream
}
```

## Rationale

Alternatives include using individual messages, e.g.
```
message StartStream {
    bytes stream_upload_id = 1;
}
message StartSegment {
    bytes stream_upload_id = 1;
    int64 segment_index = 2;
    int64 part_number = 3;
}
```
but this would result in more rpc calls overall.
Also, trying to combine a StartStream message and StartSegment message would lead to unused fields sometimes (like stream_upload_id):
```
message StartStreamAndStartSegment {
	StartStream start_stream = 1;
	StartSegment start_segment = 2;
}
```

The current upload also uses the full path every time and the request includes bucket, path, stream id, and segment index. This is less future proof because the server can't squirrel away data in the opaque upload id that clients will send back to it. With the new design, whatever the server provides for the stream_upload_id, the client has to send back. In the future we may add a feature that will require extra info from the client, so this would allow the client to respond back with stream_upload_id content.

## Implementation

- Implement the new metainfo rpcs using the same backend
    - Some requests will have to be failures (versions, multipart, etc. since they're not implemented)
- Port the clients to use the new rpcs
- Remove the old rpcs
- Slowly migrate the backend to support the new rpcs

## Open issues (if applicable)

- Jeff has no idea if some of these fields are necessary or what they are for (root_piece_id, etc.)
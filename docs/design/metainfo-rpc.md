# Metainfo RPC Service Refactor

## Background

Our current metainfo rpc service isn't very future proof.
If we shipped our current client code, it wouldn't be easy to update clients and add support for the following:
- versions
- multipart uploads
- cleanup for uncommitted segments
- general concurrency fixes

Native multipart upload can't happen with our current architecture because there's no concept of starting and ending a stream.

Versions also wouldn't work, because if concurrent clients tried to upload at the same time, they would clobber each other.
The current protobuf does not contain which version is being written, so it has to assume "latest version", which is problematic with concurrent clients (they both are writing to the "latest version").

This should also reduce the number of roundtrips needed to start and end both streams and segments.

This design should ensure compatibility between client and server in the future as these features are implemented. Neither clients nor servers will be able to access more information than what we provide in this new proto, so we need to make sure the messages and service calls anticipate needs for the above features.

## Design

The following is a proto file for just the rpc portion to create segments. The read portions just need to have a stream id included, which should default to being the largest stream id.
(When we implement versions, every upload will have a new stream, which will be given the largest stream id at that time. This will allow us to get older versions by checking the stream id.)

With this design, streams will be created that contain multiple parts. There were already RPCs to finalize individual segments, but each segment was not logically associated with a part, but instead just the specific object.

```protobuf
service Metainfo {
    rpc CreateBucket(BucketCreateRequest) returns (BucketCreateResponse);
    rpc GetBucket(BucketGetRequest) returns (BucketGetResponse);
    rpc DeleteBucket(BucketDeleteRequest) returns (BucketDeleteResponse);
    rpc ListBuckets(BucketListRequest) returns (BucketListResponse);

    rpc CreateObject(ObjectCreateRequest) returns (ObjectCreateResponse);
    rpc CommitObject(ObjectCommitRequest) returns (ObjectCommitResponse);
    rpc ListObjects(ObjectListRequest) returns (ObjectListResponse);

    rpc DeleteObject(ObjectDeleteRequest) returns (ObjectDeleteResponse);
    rpc FinishDeleteObject(ObjectDeleteRequest) returns (ObjectDeleteResponse);

    rpc CreateSegment(SegmentCreateRequest) returns (SegmentCreateResponse);
    rpc AbortSegment(SegmentAbortRequest) returns (SegmentAbortResponse);
    rpc CommitSegment(SegmentCommitRequest) returns (SegmentCommitResponse);
    rpc MakeInlineSegment(SegmentMakeInlineRequest) returns (SegmentMakeInlineResponse);

    rpc Batch(BatchRequest) returns (BatchResponse);

    rpc ListSegments(SegmentListRequest) returns (SegmentListResponse);
    rpc DownloadSegment(SegmentDownloadRequest) returns (SegmentDownloadResponse);
}

message Bucket {
    bytes        name = 1;
    CipherSuite  path_cipher = 2;
    bytes        attribution_id = 3;

    google.protobuf.Timestamp created_at = 4;
    
    int64                default_segment_size = 5;
    RedundancyScheme     default_redundancy_scheme = 6;
    EncryptionParameters default_encryption_parameters = 7;
}

message BucketCreateRequest {
    Bucket bucket = 1; // ignores created_at
}

message BucketCreateResponse {}

message BucketGetRequest {
    bytes name = 1;
}

message BucketGetResponse {
    Bucket bucket = 1;
}

message BucketDeleteRequest {
    bytes name = 1;
}
message BucketDeleteResponse {}

message BucketListRequest {
    bytes     cursor    = 1;
    Direction direction = 2;
    int32     limit     = 3;
}

message BucketListResponse {
    repeated Bucket items = 1;
    bool            more  = 2;
}

message Object {
    enum Status {
        UPLOADING;
        COMMITTING;
        COMMITTED;
        DELETING;
    }

    bytes  bucket;
    bytes  encrypted_path;
    int32  version;
    Status status;

    bytes  stream_id;

    google.protobuf.Timestamp created_at;
    google.protobuf.Timestamp status_at;
    google.protobuf.Timestamp expires_at;

    bytes  encrypted_metadata_nonce;
    bytes  encrypted_metadata;

    int64                fixed_segment_size;
    RedundancyScheme     redundancy_scheme;
    EncryptionParameters encryption_parameters;

    int64 total_size;
    int64 inline_size;
    int64 remote_size;
}

message ObjectCreateRequest {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;
}

message ObjectCreateResponse {
    bytes  bucket;
    bytes  encrypted_path;
    int32  version;

    bytes  stream_id;

    google.protobuf.Timestamp expires_at;

    bytes  encrypted_metadata_nonce;
    bytes  encrypted_metadata;

    RedundancyScheme     redundancy_scheme;
    EncryptionParameters encryption_parameters;
}

message ObjectCommitRequest {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;
}

message ObjectCommitResponse {
    Object object = 1;
}

message ObjectListRequest {
    bytes     bucket = 1;
    bytes     encrypted_prefix = 2;
    bytes     encrypted_cursor = 3;
    Direction direction = 4;
    int32     limit = 5;
    bool      recursive = 6;

    fixed32   meta_flags = 7;
    bool      include_partial = 8;
    bool      include_all_versions = 9;
}

message ObjectListResponse {
    repeated Object items = 1;
    bool more = 2;
}

message ObjectDeleteRequest {
    bytes  bucket;
    bytes  encrypted_path;
    int32  version;
}

message ObjectDeleteResponse {
    ???
}

message ObjectFinishDeleteRequest {
    bytes  bucket;
    bytes  encrypted_path;
    int32  version;
}

message ObjectFinishDeleteResponse {}


message SegmentCreateRequest {
    bytes stream_id;
    int32 part_number;
    int32 index;

    int64 max_encrypted_segment_size;
}

message SegmentCreateResponse {
    repeated AddressedOrderLimit addressed_limits;
}

message AddressedOrderLimit {
    orders.OrderLimit2 limit   = 1;
    node.NodeAddress   address = 2;
}

message SegmentCommitRequest {
    bytes stream_id;
    int32 part_number;
    int32 index;

    bytes encrypted_key_nonce;
    bytes encrypted_key;

    bytes encrypted_data_checksum;

    repeated orders.PieceHash signed_piece_hashes; // TODO: add encrypted_segment_size to piece hash
}

message SegmentCommitResponse {}


message SegmentContinueRequest {
    SegmentCommitRequest commit = 1;
    SegmentCreateRequest next   = 2;
}
message SegmentContinueResponse {
    SegmentCommitResponse commit = 1;
    SegmentCreateResponse next   = 2;
}

message BatchRequest {
    message Request {
        ObjectCreateRequest object_create;
        ObjectCommitRequest object_commit;

        SegmentCreateRequest     segment_create;
        SegmentCommitRequest     segment_commit;
        SegmentMakeInlineRequest segment_inline;
    }
    repeated Request requests;
}

message BatchResponse {
    message Response {
        ObjectCreateResponse object_create;
        ObjectCommitResponse object_commit;

        SegmentCreateResponse     segment_create;
        SegmentCommitResponse     segment_commit;
        SegmentMakeInlineResponse segment_inline;
    }
    repeated Response responses;
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
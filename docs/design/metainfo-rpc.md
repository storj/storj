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
service Metainfo {
    rpc CreateBucket(BucketCreateRequest) returns (BucketCreateResponse);
    rpc GetBucket(BucketGetRequest) returns (BucketGetResponse);
    rpc DeleteBucket(BucketDeleteRequest) returns (BucketDeleteResponse);
    rpc ListBuckets(BucketListRequest) returns (BucketListResponse);
    rpc SetBucketAttribution(BucketSetAttributionRequest) returns (BucketSetAttributionResponse);

    rpc BeginObject(ObjectBeginRequest) returns (ObjectBeginResponse);
    rpc CommitObject(ObjectCommitRequest) returns (ObjectCommitResponse);
    rpc ListObjects(ObjectListRequest) returns (ObjectListResponse);

    rpc BeginDeleteObject(ObjectBeginDeleteRequest) returns (ObjectBeginDeleteResponse);
    rpc FinishDeleteObject(ObjectFinishDeleteRequest) returns (ObjectFinishDeleteResponse);

    rpc BeginSegment(SegmentBeginRequest) returns (SegmentBeginResponse);
    rpc CommitSegment(SegmentCommitRequest) returns (SegmentCommitResponse);
    rpc MakeInlineSegment(SegmentMakeInlineRequest) returns (SegmentMakeInlineResponse);

    rpc BeginDeleteSegment(SegmentBeginDeleteRequest) returns (SegmentBeginDeleteResponse);
    rpc FinishDeleteSegment(SegmentFinishDeleteRequest) returns (SegmentFinishDeleteResponse);

    rpc Batch(BatchRequest) returns (BatchResponse);

    rpc ListSegments(SegmentListRequest) returns (SegmentListResponse);
    rpc DownloadSegment(SegmentDownloadRequest) returns (SegmentDownloadResponse);
}

message Bucket {
    bytes        name = 1;
    CipherSuite  path_cipher = 2;
    bytes        attribution_id = 3;

    google.protobuf.Timestamp created_at = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    int64                default_segment_size = 5;
    RedundancyScheme     default_redundancy_scheme = 6;
    EncryptionParameters default_encryption_parameters = 7;
}

message BucketCreateRequest {
    bytes        name = 1;
    CipherSuite  path_cipher = 2;
    bytes        attribution_id = 3;

    int64                default_segment_size = 4;
    RedundancyScheme     default_redundancy_scheme = 5;
    EncryptionParameters default_encryption_parameters = 6;
}

message BucketCreateResponse {
    Bucket bucket = 1;
}

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
    int32     limit     = 2;
}

message BucketListResponse {
    repeated BucketListItem items = 1;
    bool more  = 2;
}

message BucketListItem {
    bytes        name = 1;
    bytes        attribution_id = 2;

    google.protobuf.Timestamp created_at = 3 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message BucketSetAttributionRequest {
    bytes name = 1;
    bytes attribution_id = 2;
}

message BucketSetAttributionResponse {
}

message Object {
    enum Status {
        INVALID    = 0;
        UPLOADING  = 1;
        COMMITTING = 2;
        COMMITTED  = 3;
        DELETING   = 4;
    }

    bytes  bucket         = 1;
    bytes  encrypted_path = 2;
    int32  version        = 3;
    Status status         = 4;

    bytes  stream_id = 5;

    google.protobuf.Timestamp created_at = 6 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp status_at  = 7 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp expires_at = 8 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes  encrypted_metadata_nonce = 9;
    bytes  encrypted_metadata       = 10;

    int64                fixed_segment_size    = 11;
    RedundancyScheme     redundancy_scheme     = 12;
    EncryptionParameters encryption_parameters = 13;

    int64 total_size  = 14;
    int64 inline_size = 15;
    int64 remote_size = 16;
}

message ObjectBeginRequest {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;

    google.protobuf.Timestamp expires_at = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes  encrypted_metadata_nonce = 5;
    bytes  encrypted_metadata = 6; // TODO: set maximum size limit

    RedundancyScheme     redundancy_scheme = 7; // can be zero
    EncryptionParameters encryption_parameters = 8; // can be zero
}

message ObjectBeginResponse {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;

    bytes  stream_id = 4;

    RedundancyScheme     redundancy_scheme = 5;
    EncryptionParameters encryption_parameters = 6;
}

message ObjectCommitRequest {
    bytes  stream_id = 4;
}

message ObjectCommitResponse {
    Object object = 1;
}

message ObjectListRequest {
    bytes     bucket = 1;
    bytes     encrypted_prefix = 2;
    bytes     encrypted_cursor = 3;
    int32     limit = 4;
    bool      recursive = 5;

    ObjectListItemFlags object_flags = 6;
}

message ObjectListResponse {
    repeated ObjectListItem items = 1;
    bool more = 2;
}

message ObjectListItem {
    bytes  encrypted_path = 1;
    int32  version        = 2;
    Object.Status status  = 3;

    google.protobuf.Timestamp created_at = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp status_at  = 5 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp expires_at = 6 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes  encrypted_metadata_nonce = 7;
    bytes  encrypted_metadata       = 8;
}

message ObjectListItemFlags {
    bool metadata = 1;
)

message ObjectBeginDeleteRequest {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;
}

message ObjectBeginDeleteResponse {
    bytes  stream_id = 1;

    // TODO: should this contain a list of segments needing to be deleted?
}

message ObjectFinishDeleteRequest {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;
}

message ObjectFinishDeleteResponse {
    // TODO: should this contain a list of segments needing to be deleted when not all segements have been deleted?
}

message Segment {
    bytes stream_id = 1;
    int32 part_number = 2;
    int32 index = 3;

    bytes encrypted_key_nonce = 4;
    bytes encrypted_key = 5;

    bytes checksum_encrypted_data = 6;
    int64 size_encrypted_data = 7;

    bytes encrypted_inline_data = 8;
    repeated bytes nodes = 9 [(gogoproto.customtype) = "NodeID", (gogoproto.nullable) = false];
}

message SegmentBeginRequest {
    bytes stream_id = 1;
    int32 part_number = 2;
    int32 index = 3;

    int64 max_order_limit = 4;
}

message SegmentBeginResponse {
    bytes    segment_id = 1;
    repeated AddressedOrderLimit addressed_limits = 2;
}

message AddressedOrderLimit {
    orders.OrderLimit  limit   = 1;
    node.NodeAddress   address = 2;
}

message SegmentCommitRequest {
    bytes segment_id = 1;

    bytes encrypted_key_nonce = 2;
    bytes encrypted_key = 3;

    bytes checksum_encrypted_data = 4;

    repeated SegmentPieceUploadResult upload_result = 5;
}

message SegmentPieceUploadResult {
    int32 piece_num = 1;
    bytes node_id   = 2 [(gogoproto.customtype) = "NodeID", (gogoproto.nullable) = false];
    orders.PieceHash hash = 3;
}

// only for satellite use
message StreamID {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;

    RedundancyScheme redundancy = 4;

    google.protobuf.Timestamp creation_date = 6;
    google.protobuf.Timestamp expiration_date = 7;

    bytes satellite_signature = 4;
}

// only for satellite use
message SegmentID {
    StreamID stream_id = 1;
    int32    part_number = 2;
    int32    index = 3;

    RedundancyScheme redundancy = 4;
    bytes root_piece_id = 5 [(gogoproto.customtype) = "PieceID", (gogoproto.nullable) = false];
    repeated AddressedOrderLimit original_order_limits = 6;

    bytes satellite_signature = 7;
}

message SegmentCommitResponse {}

message SegmentMakeInlineRequest {
    bytes stream_id = 1;
    int32 part_number = 2;
    int32 index = 3;

    bytes encrypted_key_nonce = 4;
    bytes encrypted_key = 5;

    bytes checksum_encrypted_data = 6;
    bytes encrypted_inline_data = 7;
}

message SegmentMakeInlineResponse {}

message SegmentBeginDeleteRequest {
    bytes stream_id = 1;
    int32 part_number = 2;
    int32 index = 3;
}

message SegmentBeginDeleteResponse {
    bytes segment_id = 1;
    repeated AddressedOrderLimit addressed_limits = 2;
    
    // TODO: should we include here bool finished for inline segments, or should we use batching to combine SegmentBeginDeleteRequest/SegmentFinishDeleteResponse
}

message SegmentFinishDeleteRequest {
    bytes segment_id = 1;

    // TODO: check for uplink not sending order limits to storage nodes
}

message SegmentFinishDeleteResponse {}

message SegmentListRequest {
    bytes stream_id          = 1;
    int32 cursor_part_number = 2;
    int32 cursor_index       = 3;
    int32 limit              = 4;
    // TODO: is there a neater way to express cursor
}

message SegmentListResponse {
    repeated SegmentListItem items = 1;
    bool more = 2;
}

message SegmentListItem { // TODO: should we rename this to SegmentIndex and use it elsewhere?
    int32 part_number = 1;
    int32 index = 2;
}

message SegmentDownloadRequest {
    bytes stream_id = 1;
    int32 part_number = 2;
    int32 index = 3;

    SegmentListItem next = 4;
}

message SegmentDownloadResponse {
    bytes    segment_id = 1;
    repeated AddressedOrderLimit addressed_limits = 2;
}

message BatchRequest {
    oneof Request {
        BucketCreateRequest bucket_create;
        BucketGetRequest    bucket_get;
        BucketDeleteRequest bucket_delete;
        BucketListRequest   bucket_list;

        ObjectBeginRequest  object_begin;
        ObjectCommitRequest object_commit;
        ObjectListRequest   object_list;
        ObjectDeleteRequest object_delete;

        SegmentBeginRequest      segment_begin;
        SegmentCommitRequest     segment_commit;
        SegmentMakeInlineRequest segment_make_inline;

        SegmentBeginDeleteRequest  segment_begin_delete;
        SegmentFinishDeleteRequest segment_finish_delete;

        SegmentListRequest     segment_list;
        SegmentDownloadRequest segment_download;
    }
    repeated Request requests;
}

message BatchResponse {
    oneof Response {
        BucketCreateResponse bucket_create;
        BucketGetResponse    bucket_get;
        BucketDeleteResponse bucket_delete;
        BucketListResponse   bucket_list;
        BucketSetAttributionResponse bucket_set_attribution;

        ObjectBeginResponse  object_begin;
        ObjectCommitResponse object_commit;
        ObjectListResponse   object_list;
        ObjectDeleteResponse object_delete;

        SegmentBeginResponse      segment_begin;
        SegmentCommitResponse     segment_commit;
        SegmentMakeInlineResponse segment_make_inline;

        SegmentBeginDeleteResponse  segment_begin_delete;
        SegmentFinishDeleteResponse segment_finish_delete;

        SegmentListResponse     segment_list;
        SegmentDownloadResponse segment_download;
    }
    repeated Response responses;
}
```

## Rationale

The current upload also uses the full path every time and the request includes bucket, path, stream id, and segment index. This is less future proof because the server can't squirrel away data in the opaque upload id that clients will send back to it. With the new design, whatever the server provides for the stream_upload_id, the client has to send back. In the future we may add a feature that will require extra info from the client, so this would allow the client to respond back with stream_upload_id content.

## Implementation

- Implement the new metainfo rpcs using the same backend
    - Some requests will have to be failures (versions, multipart, etc. since they're not implemented)
- Port the clients to use the new rpcs
- Remove the old rpcs
- Slowly migrate the backend to support the new rpcs

## Open issues (if applicable)

- Jeff has no idea if some of these fields are necessary or what they are for (root_piece_id, etc.)
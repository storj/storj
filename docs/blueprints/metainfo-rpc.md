# Metainfo RPC Service Refactor

## Background

Our current metainfo RPC service isn't very future proof. If we shipped our current client code, it wouldn't be easy to update clients and add support for the following:

- object versioning
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
    rpc ListSegments(SegmentListRequest) returns (SegmentListResponse);
    rpc DownloadSegment(SegmentDownloadRequest) returns (SegmentDownloadResponse);

    rpc Batch(BatchRequest) returns (BatchResponse);
}

message Bucket {
    bytes   name = 1;
    string  attribution = 2;

    google.protobuf.Timestamp created_at = 3 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    int64                           default_segment_size = 4;
    pointerdb.RedundancyScheme      default_redundancy_scheme = 5;
    encryption.EncryptionParameters default_encryption_parameters = 6;
}

message BucketCreateRequest {
    bytes   name = 1;
    string  attribution = 2; 
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
    bool                    more  = 2;
}

message BucketListItem {
    bytes             name = 1;

    google.protobuf.Timestamp created_at = 2 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
}

message BucketSetAttributionRequest {
    bytes   name = 1;
    string  attribution = 2;
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
    bytes  encrypted_path = 2 [(gogoproto.customtype) = "EncryptedPath", (gogoproto.nullable) = false];
    int32  version        = 3;
    Status status         = 4;

    bytes  stream_id = 5 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];

    google.protobuf.Timestamp created_at = 6 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp status_at  = 7 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp expires_at = 8 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes  encrypted_metadata_nonce = 9 [(gogoproto.customtype) = "Nonce", (gogoproto.nullable) = false];
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
    bytes  encrypted_path = 2 [(gogoproto.customtype) = "EncryptedPath", (gogoproto.nullable) = false];
    int32  version = 3;

    google.protobuf.Timestamp expires_at = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes  encrypted_metadata_nonce = 5 [(gogoproto.customtype) = "Nonce", (gogoproto.nullable) = false];
    bytes  encrypted_metadata = 6; // TODO: set maximum size limit

    pointerdb.RedundancyScheme      redundancy_scheme = 7; // can be zero
    encryption.EncryptionParameters encryption_parameters = 8; // can be zero
}

message ObjectBeginResponse {
    bytes  bucket = 1;
    bytes  encrypted_path = 2 [(gogoproto.customtype) = "EncryptedPath", (gogoproto.nullable) = false];
    int32  version = 3;

    bytes  stream_id = 4 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];

    pointerdb.RedundancyScheme      redundancy_scheme = 5;
    encryption.EncryptionParameters encryption_parameters = 6;
}

message ObjectCommitRequest {
    bytes  stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
}

message ObjectCommitResponse {
}

message ObjectListRequest {
    bytes   bucket = 1;
    bytes   encrypted_prefix = 2;
    bytes   encrypted_cursor = 3;
    int32   limit = 4;

    ObjectListItemIncludes object_includes = 5;
}

message ObjectListResponse {
    repeated ObjectListItem items = 1;
    bool                    more = 2;
}

message ObjectListItem {
    bytes  encrypted_path = 1 [(gogoproto.customtype) = "EncryptedPath", (gogoproto.nullable) = false];
    int32  version        = 2;
    Object.Status status  = 3;

    google.protobuf.Timestamp created_at = 4 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp status_at  = 5 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp expires_at = 6 [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes  encrypted_metadata_nonce = 7;
    bytes  encrypted_metadata       = 8;
}

message ObjectListItemIncludes {
    bool metadata = 1;
}

message ObjectBeginDeleteRequest {
    bytes  bucket = 1;
    bytes  encrypted_path = 2 [(gogoproto.customtype) = "EncryptedPath", (gogoproto.nullable) = false];
    int32  version = 3;
}

message ObjectBeginDeleteResponse {
    bytes  stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
}

message ObjectFinishDeleteRequest {
    bytes  stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
}

message ObjectFinishDeleteResponse {
}

message Segment {
    bytes stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    SegmentPosition position = 2;

    bytes encrypted_key_nonce = 3 [(gogoproto.customtype) = "Nonce", (gogoproto.nullable) = false];
    bytes encrypted_key = 4;

    int64 size_encrypted_data = 5; // refers to segment size not piece size

    bytes encrypted_inline_data = 6;
    repeated bytes nodes = 7 [(gogoproto.customtype) = "NodeID", (gogoproto.nullable) = false];
}

message SegmentPosition {
    int32 part_number = 1;
    int32 index = 2;
}

message SegmentBeginRequest {
    bytes           stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    SegmentPosition position = 2;

    int64 max_order_limit = 3;
}

message SegmentBeginResponse {
    bytes    segment_id = 1 [(gogoproto.customtype) = "SegmentID", (gogoproto.nullable) = false];
    repeated AddressedOrderLimit addressed_limits = 2;
}

message AddressedOrderLimit {
    orders.OrderLimit  limit   = 1;
    node.NodeAddress   address = 2;
}

message SegmentCommitRequest {
    bytes segment_id = 1 [(gogoproto.customtype) = "SegmentID", (gogoproto.nullable) = false];

    bytes encrypted_key_nonce = 2 [(gogoproto.customtype) = "Nonce", (gogoproto.nullable) = false];
    bytes encrypted_key = 3;

    int64 size_encrypted_data = 4; // refers to segment size not piece size

    repeated SegmentPieceUploadResult upload_result = 5;
}

message SegmentPieceUploadResult {
    int32               piece_num = 1;
    bytes               node_id = 2 [(gogoproto.customtype) = "NodeID", (gogoproto.nullable) = false];
    orders.PieceHash    hash = 3;
}

// only for satellite use
message SatStreamID {
    bytes  bucket = 1;
    bytes  encrypted_path = 2;
    int32  version = 3;

    pointerdb.RedundancyScheme redundancy = 4;

    google.protobuf.Timestamp creation_date = 5  [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];
    google.protobuf.Timestamp expiration_date = 6  [(gogoproto.stdtime) = true, (gogoproto.nullable) = false];

    bytes satellite_signature = 7;
}

// only for satellite use
message SatSegmentID {
    SatStreamID stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    int32    part_number = 2;
    int32    index = 3;

    // TODO we have redundancy in SatStreamID, do we need it here?
    // pointerdb.RedundancyScheme redundancy = 4;
    bytes root_piece_id = 5 [(gogoproto.customtype) = "PieceID", (gogoproto.nullable) = false];
    repeated AddressedOrderLimit original_order_limits = 6;

    bytes satellite_signature = 7;
}

message SegmentCommitResponse {}

message SegmentMakeInlineRequest {
    bytes stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    SegmentPosition position = 2;

    bytes encrypted_key_nonce = 3 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    bytes encrypted_key = 4;

    bytes encrypted_inline_data = 5;
}

message SegmentMakeInlineResponse {}

message SegmentBeginDeleteRequest {
    bytes stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    SegmentPosition position = 2;
}

message SegmentBeginDeleteResponse {
    bytes segment_id = 1 [(gogoproto.customtype) = "SegmentID", (gogoproto.nullable) = false];
    repeated AddressedOrderLimit addressed_limits = 2;
}

message SegmentFinishDeleteRequest {
    bytes segment_id = 1 [(gogoproto.customtype) = "SegmentID", (gogoproto.nullable) = false];
    repeated SegmentPieceDeleteResult results = 2;
}

message SegmentPieceDeleteResult {
    int32               piece_num = 1;
    bytes               node_id = 2 [(gogoproto.customtype) = "NodeID", (gogoproto.nullable) = false];
    orders.PieceHash    hash = 3;
}

message SegmentFinishDeleteResponse {}

message SegmentListRequest {
    bytes stream_id                 = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    SegmentPosition cursor_position = 2;
    int32 limit                     = 3;
}

message SegmentListResponse {
    repeated SegmentListItem    items = 1;
    bool                        more = 2;
}

message SegmentListItem {
    SegmentPosition position = 1;
}

message SegmentDownloadRequest {
    bytes stream_id = 1 [(gogoproto.customtype) = "StreamID", (gogoproto.nullable) = false];
    SegmentPosition cursor_position = 2;
}

message SegmentDownloadResponse {
    bytes                        segment_id = 1 [(gogoproto.customtype) = "SegmentID", (gogoproto.nullable) = false];
    repeated AddressedOrderLimit addressed_limits = 2;
    SegmentPosition              next = 3; // can be nil
}

message BatchRequest {
    repeated BatchRequestItem requests = 1;
}

message BatchRequestItem {
    oneof Request {
        BucketCreateRequest         bucket_create = 1;
        BucketGetRequest            bucket_get = 2;
        BucketDeleteRequest         bucket_delete = 3;
        BucketListRequest           bucket_list = 4;
        BucketSetAttributionRequest bucket_set_attribution = 5;

        ObjectBeginRequest          object_begin = 6;
        ObjectCommitRequest         object_commit = 7;
        ObjectListRequest           object_list = 8;
        ObjectBeginDeleteRequest    object_begin_delete = 9;
        ObjectFinishDeleteRequest   object_finish_delete = 10;

        SegmentBeginRequest      segment_begin = 11;
        SegmentCommitRequest     segment_commit = 12;
        SegmentMakeInlineRequest segment_make_inline = 13;

        SegmentBeginDeleteRequest  segment_begin_delete = 14;
        SegmentFinishDeleteRequest segment_finish_delete = 15;

        SegmentListRequest     segment_list = 16;
        SegmentDownloadRequest segment_download = 17;
    }
}

message BatchResponse {
    repeated BatchRequestItem responses = 1;
    string error = 2;
}

message BatchResponseItem {
    oneof Response {
        BucketCreateResponse         bucket_create = 1;
        BucketGetResponse            bucket_get = 2;
        BucketDeleteResponse         bucket_delete = 3;
        BucketListResponse           bucket_list = 4;
        BucketSetAttributionResponse bucket_set_attribution = 5;

        ObjectBeginResponse          object_begin = 6;
        ObjectCommitResponse         object_commit = 7;
        ObjectListResponse           object_list = 8;
        ObjectBeginDeleteResponse    object_begin_delete = 9;
        ObjectFinishDeleteResponse   object_finish_delete = 10;

        SegmentBeginResponse      segment_begin = 11;
        SegmentCommitResponse     segment_commit = 12;
        SegmentMakeInlineResponse segment_make_inline = 13;

        SegmentBeginDeleteResponse  segment_begin_delete = 14;
        SegmentFinishDeleteResponse segment_finish_delete = 15;

        SegmentListResponse     segment_list = 16;
        SegmentDownloadResponse segment_download = 17;
    }
}
```

## Rationale

The pointer DB design uses the full path every time and the request includes bucket, path, stream id, and segment index,
which is not future proof because the server cannot easily change the way it stores and fetches data.
Similarly there are many things that need to be verified prior to committing and the uplink shouldn't be able to modify.

This design uses `stream_id` and `segment_id` fields as a way to communicate that information that needs to be carried from
one request to another. Of course, these need to be signed and timestamped by the satellite to prevent tampering. 
The `stream_id` and `segment_id` should be treated as transient and not stored in any form as they may change.

The `BatchRequest` and `BatchResponse` will allow to send a sequence of commands without waiting for an immediate response.
The nicer design would be to use streaming, however that increases the complexity of the RPC significantly.
This also poses a challenge that the ObjectBegin and SegmentMakeInline need to carry information from one call to another.
This can be implemented by leaving the corresponding `stream_id` and/or `segment_id` empty, and carried over from the last request.

## Implementation

- Implement the new metainfo rpcs using the same backend
    - Some requests will have to be failures (versions, multipart, etc. since they're not implemented)
- Port the clients to use the new rpcs
- Remove the old rpcs
- Slowly migrate the backend to support the new rpcs

## Open issues (if applicable)

- Jeff has no idea if some of these fields are necessary or what they are for (root_piece_id, etc.)
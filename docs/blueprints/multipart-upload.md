# Multipart Upload

## Abstract

This design document describes multipart upload and how it can be implemented.
Currently, there is no direct support for multipart upload on the Satellite side.
The feature is simulated on the gateway side, which requires keeping 
a lot of data in memory and doesn't achieve the full usefulness of multipart upload.

## Background

Multipart upload gives quite a few nice properties:

* Concurrent uploads,
* continuation of partial uploads, and
* out-of-order uploads.

The flow for a multipart upload, from a S3 perspective, is:

1. starts a new multipart upload,
2. the upload data is split into multiple parts,
3. for each part: send the part,
4. calls "complete multipart upload," or
5. alternatively, calls "cancel multipart upload."

There are a few important things we must keep in mind:

* the parts can have different sizes,
* during the start of the upload, the part size can be unknown,
* the parts can be uploaded from different computers,
* the parts can be uploaded out of order, and
* the parts can be reuploaded, in case of a failed upload.

## Design

We change the path handling to support listing parts uploaded in arbitrary order. When we start an upload we will create an object at the following path:

```
<project-id>/<bucket-id>/objects/<path ...>_<version> => object information (partial)
```

The object will contain a `<stream-id>` that defines the immutable content. We suffix the path with the version number to distinguish between different versions.

We don't know the part sizes, and segments cannot be arbitrarily large, which means we need to split each part into multiple segments. To uniquely find segments we'll assign them a unique number `0x<part-number>_<segment-number>` which we call the _segment position_. To calculate _`segment position = uint64(part_number) << 32 | uint64(segment_number)`_.

As an example, when we have 3 parts, each with different number of segments, we'll assign each segment a position (written in hex, `_` is used to make numbers easier to read):

```
Part 0 and 3 segments = 0x00000000_00000000, 0x00000000_000000001, 0x00000000_00000002
Part 1 and 4 segments = 0x00000001_00000000, 0x00000001_000000001, 0x00000001_00000002, 0x00000001_00000003
Part 2 and 2 segments = 0x00000002_00000000, 0x00000002_000000001
Part 3 and 3 segments = 0x00000003_00000000, 0x00000003_000000001, 0x00000003_00000002, 0x00000003_00000003
```

Segment position allows proper ordering, even when we get parts uploaded in random order. It's important to keep in mind that the "last segment uploaded" may not be the last segment of the object. Unfortunately, this also means there isn't a single continuous number sequence for the segments.

We cannot use `<segment-number>/<path>` (conceptually), because then it would be expensive to find or list all of them. To fix this issue, we shall store each stream in a separate namespace:

```
<project-id>/<bucket-id>/streams/<stream-id>/<segment position> => segment information
```

This way, we can list all the segments belonging to a single object. This single namespace for segments has other benefits, such as easily listing undeleted segments.

### Changes to data model

To support the above our data-model needs to have Objects and Segments. The following describes them in terms of protobuf definitions, however, they could be SQL tables or something else entirely. Similarly, for transitioning from old to new model, we may need some temporary adjustments.

``` protobuf



```



## Rationale

Multipart uploads can also be implemented with a "temporary location." First, the segments are uploaded to a temporary location and then, during object commit, moved to the main database. The benefit is that the database layout doesn't have to change. However, this adds more work to the satellite, and there are more ways this can fail.


## Implementation

The following requires Metainfo RPC changes to be completed since it will greatly simplify the integration.

1. Decide how to handle the database, whether to design a new database or implement a new schema.
2. Implement an appropriate interface for the database. https://github.com/storj/storj/pull/1874/files
3. Implement a backend corresponding to the interface.
4. Implement a shim layer to decide whether to use old implementation or new implementation. Old buckets would use old implementation, and new buckets would use the new implementation.
5. Implement live migration for old buckets to new data layout.
6. Once the migration is done, remove shim and old implementation.

## Open issues

* Since we need to support other databases, it might make sense to design the database such that other backends can handle it from the start. Alternatively, migrate to a new database completely rather than using postgres as a key-value store.

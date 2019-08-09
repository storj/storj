# Multipart Upload

## Abstract

This design document describes what problems multipart upload solves and how it can be implemented.
Currently there is no direct support for multipart upload on the Satellite side.
The feature is simulated completely on the gateway side, which requires keeping 
a lot of data in memory and doesn't achieve the full usefulness of metainfo upload.

## Background

Multipart Upload gives quite a few nice properties:

* Concurrent uploads,
* continuation of partial uploads, and
* out-of-order uploads.

The flow for a multipart upload, from a S3 perspective is:

1. starts a new multipart upload
2. the upload data is split into multiple parts
3. for each part: send the part
4. calls complete multipart upload
5. alternate, calls cancel multipart upload

There are few important things we must keep in mind:

* The parts can have different sizes.
* During the start of the upload, the part size can be unknown.
* The parts can be uploaded from different computers.
* The parts can be uploaded out of order.
* The parts can be reuploaded, in case of a failed upload.

## Design

These requirements lead us into a design that requires changing the data layout on the Satellite.

When we start an upload we will create an Object at the following path,
where the Object contains a `stream-id`:

```
<project-id>/<bucket-id>/objects/<path ...>_0000 => object information (partial)
```

Where `_0000` is the object version in hex.

Since there is no specific size for multipart upload parts we still need to split them into segments.
Hence, we need to split each multipart part into multiple segments.
To uniquely find them we'll assign them a unique number `<part-number>_<segment-number>`, which we call the segment position.

For example when we have 3 parts, each with different number of segments, we'll assign each segment a position (written in hex):

```
Part 0 and 3 segments = 0x00000000_00000000, 0x00000000_000000001, 0x00000000_00000002
Part 1 and 4 segments = 0x00000001_00000000, 0x00000001_000000001, 0x00000001_00000002, 0x00000001_00000003
Part 2 and 2 segments = 0x00000002_00000000, 0x00000002_000000001
Part 3 and 3 segments = 0x00000003_00000000, 0x00000003_000000001, 0x00000003_00000002, 0x00000003_00000003
```

This segment position allows to create proper ordering even when we get parts uploaded in different order or from different computers.
Note: this also means that the "last segment uploaded" may not be the last segment of the object.

This design means that there isn't a single continuos number sequence for the segemnts.
This means we cannot store them as `<index>/<path>` (conceptually) anymore, because then it would be expensive to find or list all of them.

This leads into another change, that we need to store them in a single "namespace" which we can list.

```
<project-id>/<bucket-id>/streams/<stream-id>/<segment position> => segment information
```

This way we can list all the segments belonging to a single object.

This single namespace for object segments adds other benefits, such as we can more easily detect things that still need to be deleted.

## Rationale

There is an alternate design for this, to keep the segments in a "temporary location" and then during object committing move them into the appropriate location.
This design adds a lot of more work for the Satellite, however the data-layout could stay relatively the same.

## Implementation

All of this require metainfo API changest to be completed.

1. Decide how to handle the database, whether to design new database or implement new schema.
2. Implement appropriate interface for the database. https://github.com/storj/storj/pull/1874/files
3. Implement backend corresponding to the interface.
4. Implement shim layer to decide between whether to use old implementation or new implementation. Old buckets use old implementation, new buckets use new implementation.
5. Implement live migration for old buckets to new data layout.
6. Once the migration is done, remove shim and old implementation.

## Open issues

* Since we need to support better ways for the database, it might make sense to design the database for storing this information such that we do not have to handle multiple data migrations.
* We also need to handle live-migration of the data.
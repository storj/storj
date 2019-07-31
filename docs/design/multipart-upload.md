# Multipart Upload

## Abstract

This design document describes what problems multipart upload solves and how it can be implemented. Currently there is no direct support for multipart upload on the Satellite side. The feature is simulated completely on the gateway side, which requires keeping a lot of data in memory and doesn't achieve the full usefulness of metainfo upload.

## Background

Multipart Upload gives quite a few nice properties:

* Concurrent uploads,
* continuation of partial uploads, and
* out-of-order uploads.

The flow for a multipart upload, from a S3 perspective is this:

1. starts a new multipart upload
2. the upload data is split into multiple parts
3. for each part: send the part
4. calls complete multipart upload
5. alternate, calls cancel multipart upload

There are few important things we must keep in mind:

1. The parts can have different sizes.
2. During the start of the upload, the part size can be unknown.
3. The parts can be uploaded from different computers.
4. The parts can be uploaded out of order.
5. The parts can be reuploaded, in case of a failed upload.

## Design

For storing the information we need to modify how data is stored on the satellite.

When we start an upload we will create an object at path, which internally contains a `stream-id`:

```
<project-id>/<bucket-id>/objects/<path ...> => object information (partial)
```

Since there is no specific size for parts we still need to split them into segments.
Hence for each part we get multiple segments.

For example when we have 3 parts, each with different number of segments, we'll assign each segment a position (written in hex):

```
Part 0 and 3 segments = 0x00000000_00000000, 0x00000000_000000001, 0x00000000_00000002
Part 1 and 4 segments = 0x00000001_00000000, 0x00000001_000000001, 0x00000001_00000002, 0x00000001_00000003
Part 2 and 2 segments = 0x00000002_00000000, 0x00000002_000000001
Part 3 and 3 segments = 0x00000003_00000000, 0x00000003_000000001, 0x00000003_00000002, 0x00000003_00000003
```

This way even when we get them out-of-order we can reconstruct the correct order by using lexical order. Since now we have gaps in the segments, we need to store them in a single namespace, meaning:

```
<project-id>/<bucket-id>/streams/<stream-id>/<segment position> => segment information
```

That way we can list all the segments belonging to a single object. For encoding we can use the ideas from [Path Component Encoding](path-component-encoding.md), to ensure we can properly list things.

This design implies we cannot have special handling for last segment, since the "last-segment" can be the first thing to be uploaded.

## Rationale

Instead of creating the custom segment we could also upload to a "temporary" location and then during commiting of object write the segments into the pointerdb, however this adds more pressure to the server. This design ensures we don't have to move data to commit objects.

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

* Should we implement the new metainfo database together with this rather than changing the pointerdb to fit these needs?
* Figure out live-migration of data.
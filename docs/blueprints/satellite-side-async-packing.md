# Satellite Side Async Packing

## Abstract

This blueprint describes a way to pack small objects without backwards
incompatible changes on the uplink side and should be relatively easy
to implement.

The rough idea is to have a background process that packs together
small object segments into a single segment. Then the objects will
refer to a single segment.

## Background

Small files have several disadvantages in the current system.

- Every small file requires one object and one segment.
- Every small piece on storage node uses more disk than needed.
- Metabase segment loop has a lot of things to iterate.
- There is a significant connection overhead for small files. _This design does not improve this._

## Design

In metabase database, multiple objects will refer to a single stream (segment),
with sub-ranges. This implies that the segments table will now need to contain
a reference count among other things.

A packing background process will:

1. query the objects table one bucket at a time,
   to discover small objects that haven't been packed together;
2. download encrypted object data into memory;
3. rebuild pieces as necessary;
4. concatenate the pieces together, keeping track where each segment is located
   with `stream_offset` and `stream_length`.
5. upload the packed segment to a set of storagenodes
6. replace the stream references in the objects table, with the packed segments

The satellite, on a download request, will add the `stream_offset` and
`stream_length` to the `OrderLimit`. The uplink doesn't need to be aware of this
change. The storagenode when getting such an order limit, will
appropriately only read a subrange from the stored piece.

Note, we should not delete the old segments at step 5., otherwise we might
delete a segment that is being actively downloaded. We need to preserve old
unpacked segments at least the duration of order limit validity duration
(currently 48h). This could be achieved by updating segment TTL-s on the
storagenode.

Repair, audit, project accounting may need adjustments. Storage node accounting
and GC should work as is.

We also need a process to handle pack fragmentation. For example when 99% of
packed segment is "deleted", then we should repack. One option to implement this
is to add a process that iterates packed segments and looks at the
"garbage_bytes" -- however there's no easy way to mark the "objects"
as needing repacking.

The reference counting, of course, would add overhead to all deletes.

## Design - Uplink

It would be possible to implement additional API to uplink that uploads packed
segments from the start. A very rough implementation could look like:

```
pack := uplink.BeginPack()
for _, file := range files {
	err := pack.Upload(file.Name, ..., file.Data)
	if err != nil {
		return err
	}
}
pack.Commit()
```

This is not the final design, it will be revisited when implementing the API.

The main problem with this approach is how to handle going across segment
boundaries, since we don't want a `2 byte` object to be stored on two different
segments.

The easiest way to avoid boundary issue is to force the user to
specify the size upfront. e.g. taking []byte as argument or taking a size
as an argument for starting the upload.

The alternative is to over-allocate bandwidth and set a limit for split
position. e.g. when packed segment is over 32MiB then break whenever the
current object is finished. When the object is actually large, then split
the segment as usual.

## Rationale

It's possible to implement the same packing across all buckets and projects,
however this would have signifcant issues with deleting whole buckets and
projects. When deleting a bucket or project we would be able to directly
delete the segments.

This satellite side packing does have an ingress and egress cost, however it
should outweigh the long-term satellite-side storage cost.

## Implementation

### Storage Node

Add `stream_offset` and `segment_length` to `pb.OrderLimit`. The storage node
should respect these values, and uplink should not need to treat such order
limits separately.

### Satellite

#### Pack upload selection

For uploading packed segments we should only upload to storage nodes
that support the `stream_offset` feature.

#### Metabase changes

Update metabase tables:

- add `stream_offset`, `stream_length` to `objects` table, to track the
  location in the stream / segment;
- add `stream_encrypted_key_nonce`, `stream_encrypted_key` etc. to `objects` table,
  to track necessary information for decryption;
- add `reference_count` to `segments` table, to track how many objects are
  still referencing a particular segment
- add `garbage_bytes` to `segments` table, to track how fragmented a given
  packed segment is.

Object deletes need to update `reference_count` and `garbage_count`.

The `stream_encrypted_key` etc. could also be stored either in the segments
table as a separate field. Or even interleaved in the segments themselves.
The appropriate location should be tested.

New API will be needed to:

- find a batch of objects to pack,
- replace the objects with a pack.

The replacing of objects should assume that the replacing the pack partially
may fail due to concurrent updates or deletes. The packing should still succeed
when most of the replacements succeed.

We need to take care that we don't break dowloads while replacing the streams
in objects. Currently, stream_id is being used for different identity purposes.

It's quite likely we'll eventually hit a situation where:

1. uplink starts downloading object X, with piece id Y
2. satellite repacks X to a new segment
3. satellite sends deletes to storagenodes with piece id Y
4. uplink fails to download object X

The satellite needs to take care that the piece id Y is stored at least
until the downloading "token" expires.

#### Packing Process

The packing process will need to:

1. query the objects table one bucket at a time,
  to discover small committed objects that haven't been packed together;
2. download encrypted object data into memory;
3. rebuild pieces as necessary;
4. concatenate the pieces together, keeping track where each segment is located
  with `stream_offset` and `stream_length`.
5. upload the packed segment to a set of storagenodes
6. replace the stream references in the objects table, with the packed segments

For choosing `expires_at` in segments table we can use the maximum of the given
segments. The object will still track the appropriate `expires_at` date and
during zombie deletion the `garbage_bytes` can be updated on the segment as
necessary.

When a segment has an `encrypted_etag` we can handle these in a few possible
ways. First, we can ignore them; since the packing only applies to small
objects, which usually wouldn't be uploaded via multipart upload. Secondly,
since the etag-s are only relevant for multipart upload, we could drop them
once we commit (assuming S3 doesn't need them later).

The object packing should take into account:

- total size of the pack - choosing too few objects means the packing
  is less effective,
- expires_at time - choosing similar values means the segment needs to
  hold less garbage,
- encrypted object key - choosing objects with the same prefix are more
  likely to be deleted together,
- upload time - things uploaded at similar times are more likely to
  be deleted at similar times.

None of these are critical for the first implementation, however, we
should try to figure out good heuristics for packing to reduce fragmentation.

It would be possible to avoid satellite-side bandwidth overhead by letting
storagenodes send the pieces to each other and constructing the packed
segment / piece that way. This however has significantly more complicated
protocol, as we've seen from graceful exit.

Note, the object packing creates a strong long-tail cancellation and locality
preference towards the repacking or repair node. Hence, the repacking and repair
nodes need to be more lenient with regards to long-tail cancellation.

#### Re-packing Process

The re-packing process will need to mark `objects` as needing repacking.

By inspecting the `garbage_bytes` and `encrypted_size` in segments, it's
possible to decide whether a segment should be repacked.

TODO: figure out how to mark objects as needing repacking.

#### Service updates

TODO: review how services need to be updated.

## Alternative Solutions

### Storagenode Cohorts

One alternative approach would be to keep a "appendable segment" and during
upload choose a segment to append to. This avoids the need for a background process.

To implement, it would need some sort of in-memory index to choose appropriate
appendable segment, based on the aspects mentioned in "Packing Process". This
appropriate selection is not complicated to implement.

However, the failure scenarios are much more difficult to handle correctly.

First the storagenode now needs to have a notion of "segment being modified".
This modification can happen during ongoing downloads. Similarly, flushing
data to a [disk is non-trivial](https://www.sqlite.org/atomiccommit.html).
The current model of "write, flush, commit" without modification helps to
avoid many such problems.

The satellite then needs to take into account storage nodes failing. The
failure could be due to network issue, node restart, long tail cancellation.
When the uplink receives such failure during upload, the whole "packed
segment" so far would need to be discarded on that storage node. When the
uploads are spaced more apart in time, there's more likely that during
each "append" one node fails. Tracking additional information for partial
uploads would defeat the purpose of packing. Or in other words -- the failure
to upload to a cohort multiplies over time. This means you probably can
upload to a cohort some maximum number of times, before it would need to be
repaired.

The appendable segments also will need to be audited and repaired, complicating
the situation further.

## Wrapup

[Who will archive the blueprint when completed? What documentation needs to be
updated to preserve the relevant information from the blueprint?]

## Open issues

- Marking objects for repacking
- Concurrency issue with upload and packing
- Services that need updating.
- Calculations and simulation of the ingress/egress cost.

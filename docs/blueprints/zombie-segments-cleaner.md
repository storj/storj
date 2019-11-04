# Zombie Segments Cleaner

## Abstract

This document describes a design for cleaning segments that are not accessible with standard operations like listing.

## Background

Currently with a multisegment object, we have several places where we can leave unfinished or broken objects:
* if upload process will be stopped, because of an error or canceling, when one or more segments are already uploaded we can become zombie segments.
* if delete process will be stopped, because of an error or canceling, when one or more segments are already deleted we can have an object with an incomplete number of segments.
* in the past we had a bug where error during deleting segments from nodes was interrupting deleting segment from satellite, and we have segments available on satellite but not on storage node.
* deleted bucket can leave all objects inaccessible ???
* deleted project can leave all objects inaccessible ???

We need a system for identifying objects with missing segments, and zombie segments that are not listed as part of objects, that cleans up those segments.

In the long term, the metainfo refactor will fix this. We need a short term solution.

## Design

How detect zombie segment:
* segment doesn't have corresponding last segment (`l`)
* segment where index is greater than 0 but any previous segment is missing
* last segment (`l`) where unencrypted number of segments is greater than 0 but rest of segments are missing or number of existing segments is different from stored value

Different kind of bad segment is a case where all segments are available on satellite but were deleted from storage nodes.

General idea is to register a new `metainfo.Observer` to verify all segment paths. During iteration, each segment path will be processed and assigned to a struct that will collect segments from the same object. When the last segment will be reached or iteration will be finished each object will be checked if it contains all segments (according to rules from the beginning of this part). If yes, then helper struct will be removed from memory. If not, then all related segments will be moved to delete.

Each path processing should be done as much in asynchronous way as possible to avoid blocking metainfo loop.

## Alternative

In case of performance issues with metainfo observer as an alternative, we can detect zombie segments by iterating on a backup of pointerDB. Result of such operation would be a static list of segments to delete on production satellite.

## Implementation

Code should be added to satelite in package `satellite/segcleaner`.

Proposal for keeping segments structures:
```
// key will represent projectID/bucketName/encryptedPath
map[string]Object

type Object struct {
    segments []int32
}
```

## Open issues (if applicable)

* current metainfo.Loop implementation doesn't notify about the end of iteration
* how detect segments deleted from storage nodes but existing on satellite?
* should we try also to delete zombie segments from storage nodes or leave it for garbage collection?
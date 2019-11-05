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
**Note**: Exception from this case are segments that are part of object that is currently uploaded. For time of uploading all object segments have indexes `s0..sN` but at the end of upload (`CommitObject`) segment `sN` is renamed (`delete` and `put`) to last segment (`l`).
* segment where index is greater than 0 but any previous segment is missing
* last segment (`l`) where unencrypted number of segments is greater than 0 but rest of segments are missing or number of existing segments is different from stored value

Different kind of bad segment is a case where all segments are available on satellite but were deleted from storage nodes.

General idea is to create cli command to verify all segment paths and delete bad segments. With this command user should be able to specify flags like DB connection string, dry run (only listing bad segments) and how old segments should be verified. During iteration, each segment path will be processed and assigned to a struct that will collect segments from the same object. All segments from object where at least one segment is not old enough should be skipped. When the last segment (`l`) will be reached or iteration will be finished each object will be checked if it contains all segments (according to rules from the beginning of this part). If yes, then helper struct will be removed from memory. If not, then all related segments will be moved to delete or only printed in case of dry run.

Command can be run agains production database or in case of performance concerns agains backup of pointerDB.

## Implementation

Code should be placed in package `cmd/segment-reaper`.

Implementation can incorporate `metainfo.PointerDB` and `Iterate` method to go over all segments in DB.

Proposal for keeping segments structures:
```
// key will represent projectID/bucketName/encryptedPath
map[string]Object

type Object struct {
    // big.Int represents a bitmask of unlimited size
    segments big.Int
    // if skip is true segments from object should be removed from memory 
    // when last segment is found or iteration is finished
    skip     bool 
}
```

## Open issues (if applicable)

* how detect segments deleted from storage nodes but existing on satellite?
* should we try also to delete zombie segments from storage nodes or leave it for garbage collection?
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

We should create two cli commands. One for detecting bad segments, second one for deleting segments reported by first comamnd.

The first command for detecting bad segments should iterate over pointerDB and verify each segment. The command should allow specifying flags like DB connection string, and how old segments should be verified. During processing, segment should be assigned to a struct that will collect segments from the same object. All segments from an object where at least one segment is not old enough should be skipped. When the last segment (`l`) will be reached or iteration will be finished each object will be checked if it contains all segments (according to rules from the beginning of this part). If yes, then helper struct will be removed from memory. If not, then all related segments should be printed out. Each entry in such a report should contain segment path, creation/modification date. This command should be **ONLY** executed against pointerDB snapshot to avoid problems with constant changes in the production database.

The second command for deleting bad segments should use the result of first command. The command should allow specifying flags like DB connection string, file with results from the first command, and dry run mode. Each reported segment should be verified if it's not changed since detection report was done. If segment was not changed it should be deleted from pointerDB. Verification and deletion should be done as atomic operation to avoid race conditions. Command should print the report at the end: number of deleted segments, number of segments skipped because of being newer than reported, skipped segments paths. Execution with dry run flag should only print results but without deleting segments from the database. This command should be executed against production database.

## Non Goals

* deal with segments inaccessible because of bucket deletion
* deal with segments inaccessible because of project deletion

## Implementation

Code should be placed in package `cmd/segment-reaper`. The first command for detecting bad segments should be named `detect`. The second command for deleting bad segments should be named `delete`.

Implementation can incorporate `metainfo.PointerDB` and `Iterate` method to go over all segments in DB. 

Proposal for keeping segments structures:
```
// key will represent projectID/bucketName/encryptedPath
map[string]Object

type Object struct {
    // big.Int represents a bitmask of unlimited size
    segments big.Int
    // if skip is true segments from object should be removed from memory when last segment is found 
    // or iteration is finished,
    // mark it as true if one of the segments from this object is newer then specified threshold
    skip     bool 
}
```

Output of `segment-reaper detect` command should be CSV with list of segments. Each row will contain project ID, segment index, bucket name, encoded encrypted path (encoded with base64 or base58) and creation/modification date. Encrypted path should be encoded to avoid printing invalid characters. 

Example:
```
    projectID;segmentIndex;bucketName;encoded(encrypted_path1);creation_date
    projectID;segmentIndex;bucketName;encoded(encrypted_path2);creation_date
    projectID;segmentIndex;bucketName;encoded(encrypted_path3);creation_date
```

Two major steps for deleting segment is verification if the segment is newer than detected earlier and delete segment in one atomic operation. For deleting segment we should use `CompareAndSwap` method. Sample code:
```
    // read the pointer
    pointerBytes, pointer, err := s.GetWithBytes(ctx, []byte(path))
    if err != nil {
        return err
    }
    // check if pointer has been replaced
    if !pointer.GetCreationDate().Equal(creationDateFromReport) {
        // pointer has been replaced since detection, do not delete it.
        return nil
    }
    // delete the pointer using compare-and-swap
    err = s.DB.CompareAndSwap(ctx, []byte(path), pointerBytes, nil)
    if storage.ErrValueChanged.Has(err) {
        // race detected while deleting the pointer, do not try deleting it again.
        return nil
    }
    if err != nil {
        return err
    }
```

## Open issues (if applicable)

* how detect segments deleted from storage nodes but existing on satellite?
* should we try also to delete zombie segments from storage nodes or leave it for garbage collection?
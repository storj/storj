# Forgotten Deletes

## Abstract

In the event of a node being audited for a piece which the satellite previously asked it to delete via delete pieces request, the node will present a signed message proving the legitimacy of the deletion. The node is not penalized and the object is deleted. We eliminate the potential of forgotten garabage collection deletes entirely.

## Background

In the event of a DB restoration from backup, it may be possible for the satellite to restore pointers for objects which were deleted, or revert pointers to a previous state containing nodes which no longer have those pieces. In this situation, the satellite would audit nodes for pieces which they were told to delete which may result in unfair disqualifications.

## Design

The solution to this problem is twofold, as there are two ways in which pieces are deleted: delete pieces requests and garbage collection.

### Delete pieces requests

Delete pieces requests are issued when a user deletes an object. The metainfo service deletes the object's pointers and then sends delete pieces requests to each of the reliable nodes which held pieces. The rest of the nodes' pieces will be eventually deleted by garbage collection. 

To prove to the satellite that a piece was deleted legitimately, we will have it sign the delete pieces requests. The node will present these to the satellite in the event of an audit for a deleted piece.

```
message DeletePiecesRequest {
    repeated bytes piece_ids = 1 [(gogoproto.customtype) = "PieceID", (gogoproto.nullable) = false];
    bytes satellite_signature = 2;
}
```

The storage nodes will serialize and save these signed messages into a file store.

#### File store

// TODO: I need to rework this. I'm pushing these changes just because. I actually don't think this 24 hour file thing will do anything for us. If we change GC so that those pieces will never be reverted, we'll know when to delete these messages when the GC retain request does not contain any of its pieces.

This file store will essentially consist of two directories: one for piece IDs and one for signed delete messages. Piece IDs will be sorted into buckets by prefix and signed delete messages are sorted into files by 24 hours. 

When a delete request comes in, we will append the length of the request and the request itself to today's file in the corresponding satellite directory and return the offset at which the data was written.

Then, for each deleted piece we will append the piece ID and the filename and offset of the signed delete request to the corresponding file based on its prefix. The filename and offset will need to be a fixed number of bytes.

When we need to pull up the signed message as proof, we can search for the requested piece ID based on its prefix. If a file corresponding to the piece ID prefix exists, we scan the file to locate the piece ID. If the piece ID is found, we read the filename and offset pointing to the signed delete message. If the piece ID is not found in the file, or no prefix file exists, then there is no signed delete message to retrieve.

#### Submitting proof to the satellite

To avoid failing the audit, we want the node to be able to provide proof of the delete request to the satellite right away. To do this we will add the signed DeletePiecesRequest to the PieceDownloadResponse protobuf.

```
message PieceDownloadResponse {
    // Chunk response for download request
    message Chunk {
        int64 offset = 1;
        bytes data = 2;
    }
    Chunk chunk = 1;
    orders.PieceHash hash = 2;
    orders.OrderLimit limit = 3;
    
    DeletePiecesRequest delete_pieces_request = 4;
}
```

When the satellite receives a PieceDownloadResponse and DeletePiecesRequest is not nil, it will verify the signature and check for the corresponding piece ID. If these checks pass, the audit will not count as a failure. Further, upon receipt of the first verified DeletePiecesRequest, this means that the entire object to which this segment belongs should not exist and we should delete it.

### Garbage collection

Garbage collection works by observing the metainfo loop. This collects all the piece IDs which a node should have according to the pointer DB at that time. The satellite then sends these piece IDs to the storage node, indicating that it should keep these pieces and all pieces which were created after the creation of the bloom filter and delete the rest.

Instead of proving the pieces were deleted, we will remove the possibility of these pieces being restored.
To acheive this we will simply run garbage collection from a DB snapshot at a point in time beyond which we will never restore. That is, we must decide what is the furthest possible point the past we may restore to and only run garbage collection on a snapshot at or before this point. If this is guaranteed, no pieces deleted by garbage collection should ever be reverted by a DB restoration.

## Rationale

### Alternate Approaches

- No piece delete requests - only garbage collection

    Given the approach above for running garbage collection on snapshots beyond which we will not restore, if this were the only way pieces were deleted we would not have to account for forgotten deletes at all. From a code-complexity perspective this is the ideal solution. However, the drawback is that nodes must already keep trash pieces around for 7 days before they are permanently removed. Now they will have to keep pieces around for an additional X days after the pointer has already been deleted before they receive the message to move the pieces into the trash. Piece delete requests will at least cut back on the amount of trash. However, if the amount of pieces deleted by garbage collection heavily outweighs what is deleted by piece delete requests, the benefit may be small, and we should consider delegating all deletes to garbage collection.

- Signing bloom filters

    A signed bloom filter does not give us enough information. All it tells us is that at some point in time, the satellite did not expect the node to have a particular piece. Whether the bloom filter was issued before or after the piece was deleted, or that the node was removed from the piece, may not be easy to determine. If nodes were to present their most recently acquired bloom filters, this would clearly indicate that the node should not have said piece. However, given that the entire problem is precipitated by a satellite having amnesia, there would be no way to verify that a given bloom filter is the most recently issued. Further, the forgetful satellite may even issue a bloom filter which contains the false piece before the node is audited for it. We may be able to give nodes the benefit of the doubt by allowing them to present a bloom filter issued within the last X days which does not contain the requested piece ID. However, again, there may be a large lag between the time of the database restoration and the time the node is actually audited for the piece. We would need to allow the node to present very old bloom filters, which may allow widespread cheating.

- Satellite signs a delete request for each piece ID

   Currently the satellite signs a batch of piece IDs for deletion. If instead the satellite signs each individual piece ID, this would make storing and retrieving proof of deletion much easier. We could just store each piece ID and signed proof in a single DB row. However, the extra overhead of signing each piece may be prohibitive.

- Storing the signed delete request in place of the piece data

    I like this idea. However, we would need to figure out that this is actually a deleted piece and the data should be sent in the appropriate response field in order for the satellite to process it as such. In this case I think we would need to check every single download request to see if it is a deleted piece, which seems too inefficient. Perhaps if we stored these in a separate directory than blobs it would work. That way, when we fail to find the piece in blobs, we could double check this deleted piece proofs directory. If it exists, we send the data in the response field for signed delete requests. However, as one delete request contains multiple piece IDs, storing the message with each individual piece would result in a lot of replicated data. If each individual piece for deletion were signed this would not be a problem, but as mentioned above, I am unsure if spending the resources to sign each piece would be prohibitive.

## Implementation

1. Add new field, satellite_signature, to DeletePiecesRequest protobuf

2. Add new field, delete_pieces_request, to PieceDownloadResponse protobuf

3. Add code for satellite to sign DeletePiecesRequests

4. Implement file store for piece IDs and signed delete requests

5. Add code to check for and return PieceDeleteRequest if piece cannot be found during download

6. Add code to satellite to delete object if PieceDownloadResponse contains a valid DeletePiecesRequest

7. Determine the furtherst point in the past we are allowed to revert to. If we are not already doing so, we need to automate taking snapshots of the pointer DB and make sure we run garbage collection on the appropriate one.

8. Garbage collection retain messages currently set creationDate to the current time. This value is used to determine which pieces on the storage node should not be deleted because they are new and were not seen by the piece tracker. This value will need to be set to the time of the DB snapshot on which we are running garbage collection. Otherwise, the storage nodes will delete all pieces created since the DB snapshot.

## Wrapup

- We may need to edit documents regarding piece deletion/garbage collection

## Open issues

- The design laid out in this document addresses the issue of falsely punishing nodes due to a DB restoration. It does not however, solve all data loss problems that could arise from a DB restoration. For example, how will we handle forgotten uploads?

- The solution to forgotten garbage collection deletes is going to increase the TTL of trash that nodes hold. They will probably not like that. The extent to which the TTL of garbage is increased depends on how far back in time we may want to restore.


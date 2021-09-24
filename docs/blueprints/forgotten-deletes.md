# Forgotten Deletes

## Abstract

In the event of a node being audited for a piece which the satellite previously asked it to delete via delete pieces request, the node will present a signed message proving the legitimacy of the deletion. The node is not penalized and the object is deleted. We eliminate the potential of forgotten garabage collection deletes entirely.

## Background

In the event of a DB restoration from backup, it may be possible for the satellite to restore pointers for objects which were deleted, or revert pointers to a previous state containing nodes which no longer have those pieces. In this situation, the satellite would audit nodes for pieces which they were told to delete which may result in unfair disqualifications.

## Design

The solution to this problem is twofold, as there are two ways in which pieces are deleted: delete pieces requests and garbage collection.

### Garbage collection

Garbage collection works by observing the metainfo loop. This collects all the piece IDs which a node should have according to the pointer DB at that time. The satellite then sends these piece IDs to the storage node, indicating that it should keep these pieces and all pieces which were created after the creation of the bloom filter and delete the rest.

Instead of proving the pieces were deleted, we will remove the possibility of these pieces being restored.
To acheive this we will simply run garbage collection from a DB snapshot at a point in time beyond which we will never restore. That is, we must decide what is the furthest possible point the past we may restore to and only run garbage collection on a snapshot at or before this point. If this is guaranteed, no pieces deleted by garbage collection should ever be reverted by a DB restoration.

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

// TODO: directory names
Within the storage node's 'storage' directory, we will add a new directory called 'deleted-pieces-proof'. Within this directory we will create a subdirectory for each satellite, and within each of these we will add two more directories: 'pieces' and 'signed-requests'.

##### Creating Proofs

The DeletePieces method on the piecestore endpoint is where the the delete pieces requests are sent. From here, the piece IDs within the message are enqueued for the piece deleter job to delete. Before the pieces are enqueued, we will write the serialized message into a file in the 'signed-requests' directory of the respective satellite, skipping the first byte which we will reserve. Then we pass this filepath along with the piece IDs into the Enqueue method. As pieces in the queue are deleted, we create a new path for them in the 'pieces' directory. At this path, we write a text file containing the name of the signed delete request file which contains this piece. Then, we open the signed delete request file and increment the first byte.

##### Deleting Proofs

We can use garbage collection to determine when to remove the deleted piece IDs and their signed proofs. Due to the aforementioned changes to how we run garbage collection, we can be sure that any piece which does not exist in the filter will not be reverted. Thus, once the filters no longer contain the deleted pieces, the piece ID paths can be removed. To do this, we will have the 'Retain' code path walk and clean up the deleted pieces directory just as it does the 'blobs' directory. As these piece ID paths are deleted, we will read the signed delete message files that they point to and decrement the first byte. Once this value reaches zero, none of the deleted pieces in the message can be reverted by a DB restoration; the proof is no longer necessary, and the file can be deleted.

#### Submitting proof to the satellite

To avoid failing the audit, we want the node to be able to provide proof of the delete request to the satellite right away. To do this we will add the signed DeletePiecesRequest to the PieceDownloadResponse protobuf. If a piece cannot be found during a download, we will search for it in 'storage/deleted-pieces-proofs/satelliteID/pieces'. If it is found, we read the file at this location to get the filename of the signed delete request in 'storage/deleted-pieces-proofs/satelliteID/signed-requests' which contains this piece ID. We read this file, skipping the first byte to get the signed request, pass the data into the PieceDownloadResponse and send it to the satellite.

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

When the satellite receives a PieceDownloadResponse and DeletePiecesRequest is not nil, it will verify the signature and check for the corresponding piece ID. If these checks pass, the audit will not count as a failure. Further, upon receipt of the first verified DeletePiecesRequest, this means that the entire object to which this segment belongs should not exist, and we should cancel any remaining downloads and delete the object.

## Rationale

### Alternate Approaches

- No piece delete requests - only garbage collection

    Given the approach above for running garbage collection on snapshots beyond which we will not restore, if this were the only way pieces were deleted we would not have to account for forgotten deletes at all. From a code-complexity perspective this is the ideal solution. However, the drawback is that nodes must already keep trash pieces around for 7 days before they are permanently removed. Now they will have to keep pieces around for an additional X days after the pointer has already been deleted before they receive the message to move the pieces into the trash. Piece delete requests will at least cut back on the amount of trash. However, if the amount of pieces deleted by garbage collection heavily outweighs what is deleted by piece delete requests, the benefit may be small, and we should consider delegating all deletes to garbage collection.

- Signing bloom filters

    A signed bloom filter does not give us enough information. All it tells us is that at some point in time, the satellite did not expect the node to have a particular piece. Whether the bloom filter was issued before or after the piece was deleted, or that the node was removed from the piece, may not be easy to determine. If nodes were to present their most recently acquired bloom filters, this would clearly indicate that the node should not have said piece. However, given that the entire problem is precipitated by a satellite having amnesia, there would be no way to verify that a given bloom filter is the most recently issued. Further, the forgetful satellite may even issue a bloom filter which contains the false piece before the node is audited for it. We may be able to give nodes the benefit of the doubt by allowing them to present a bloom filter issued within the last X days which does not contain the requested piece ID. However, again, there may be a large lag between the time of the database restoration and the time the node is actually audited for the piece. We would need to allow the node to present very old bloom filters, which may allow widespread cheating.

- Why the indirection from piece ID to signed delete requests?

    It is possible for us to store the signed delete request directly at the piece ID path. However, due to the fact that each request may contain multiple piece IDs, we would be replicating the data at each piece ID path. If each individual piece for deletion had its own signature we would be able to directly store the data without a problem.

- Satellite signs a delete request for each piece ID

   Currently the satellite signs a batch of piece IDs for deletion. If instead the satellite signs each individual piece ID, this would make storing and retrieving proof of deletion much easier, as mentioned above. However, the extra overhead of signing each piece may be prohibitive.

- Store pointer to signed proof at piece location in 'blobs' directory

    It may be possible for us to store the pointer to the signed proof at the piece location in the blobs directory rather than a separate 'deleted-pieces' directory. In this case we would need to implement some way of distinguishing deleted pieces from regular pieces, such as a special file extension. The issue I'm seeing is that a fundamental component of the 'pieces' system is the storage.FormatVersion. All pieces have a specific format version which tells us how to interpret the data. Format versions start at 0 and increment as new versions are added. Should deleted pieces then constitute a new format version? It doesn't look like we should do that. When creating a new piece, it is always created using the highest defined version. We don't want to store newly uploaded pieces with the version for deleted pieces! 
    Simply leaving the value blank doesn't really solve the problem either, as 0 is a specific version already. So we would need to pass along another value in the code, perhaps a boolean indicating that this is deleted piece data, in addition to the format version in order to process deleted piece data correctly, but I have a feeling this approach will result in clutter. Here's a hacky idea: what if we did define a new format version for deleted piece data, but set it to -1? This would circumvent some of the problems with format versions. Though, I'm not sure if this approach holds any substantial benefits over using a separate directory.

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

- If we guarantee that we will run garbage collection at a point we will never revert beyond, does this mean that we may not be able to restore trash on the storage nodes? For example, imagine a bug in the zombie segment reaper which deletes valid segments, and maybe we only find out after garbage collection has run. The nodes may still have those pieces in the trash and could restore them, but we're not supposed to revert the database at this point.
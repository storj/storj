# Repair using Piece Hashes

## Abstract

The satellite should repair files also using piece hashes to minimize CPU and bandwidth load.

## Background

The white-paper states:

> Data repair is an ongoing, costly operation that will use significant bandwidth, memory, and processing power, often impacting a single operator. As a result, repair resource usage should be aggressively minimized as much as possible.
> 
> For repairing a segment to be effective at minimizing bandwidth usage, only as few pieces as needed for reconstruction should be downloaded. Unfortunately, Reed-Solomon is insufficient on its own for correcting errors when only a few redundant pieces are provided. Instead, piece hashes provide a better way to be confident that we’re repairing the data correctly.
> 
> To solve this problem, hashes of every piece will be stored alongside each piece on each storage node. A validation hash that the set of hashes is correct will be stored in the pointer. During repair, the hashes of every piece can be retrieved and validated for correctness against the pointer, thus allowing each piece to be validated in its entirety. This allows the repair system to correctly assess whether or not repair has been completed successfully without using extra redundancy for the same task.

Hash verification on the satellite requires understanding the current piece signing and verification workflow:

1. Satellite generates a new key-pair for each piece.
2. The piece public key is included in the order limit and signed by the satellite
3. The piece private key is included in the create segment, download segment or delete a segment.
4. The uplink uses piece private key to sign the orders and uplink piece hash.
5. The storage node verifies order limit signature as usual and then uses the piece public key to verify the order and piece hash.
6. Storage node sends both the order limit and order to the satellite during settlement.

Thus the uplink-signed `uplink_piece_hash` is already stored in the storage node's `pieceinfo_` table as a serialized protocol buffer and includes the `PieceID`, `Hash`, and uplink `Signature`. The `pieceinfo_` table also includes the `order_limit` which contains the `PiecePublicKey` necessary for validating the `PieceHash`, as well as the `SatelliteSignature` to validate the `OrderLimit` itself.

## Design

When a satellite downloads pieces for repair, the storage node should also include the `PieceHash` and `OrderLimit` to the satellite along with each piece.

The satellite should attempt to download exactly the Reed-Solomon minimum number of pieces during a repair, storing them in memory. It should validate the `OrderLimit` using its own private key.  It should validate that the `PieceHash` `Signature` corresponds to the `PublicKey` contained in the `OrderLimit`.  The satellite should calculate the size and hash of the piece as it downloads them, and it should be equal to the `PieceHash` `Hash` and `PieceSize`. If a piece fails any of these checks, an error should be logged and a different piece should be downloaded.  Once all pieces are downloaded, the `PiecePublicKey` of all pieces should be compared and equal.  If they are not equal, an error should be logged and more pieces should be downloaded until a minimum number of pieces share the same `PiecePublicKey`.  When enough pieces have been downloaded and validated, Reed-Solomon decoding should be used to reassemble the segment, and repair should continue as previously designed.

Failed hash, signature, or public key checks may indicate cheating. Therefore audit must implement this same piece hash check logic. Changes to audit and the reporting of cheaters are otherwise outside the scope of this document.  

## Rationale

Both download and Reed Solomon decoding create more load for a satellite than hashing, so hashing should be preferred over downloading even a single piece.

Downloading for repair is significantly different enough from streaming as to warrant a new, custom implementation.

Using only the minimum number of pieces means that Reed-Solomon does not act as a check during repair. Hence hashing is used instead. While an uplink could potentially send signed bogus data to a storage node, the storage node would not be penalized by these actions. This requires that Audit implements a similar piece hash check instead of relying solely on Reed-Solomon encoding.

The size of all piece hashes downloaded should be roughly equal to a default maximum segment size : 64MiB.  It seems preferable to keep this in memory over dealing with persistance to disk.

## Implementation

1. Add an optional `PieceHash` and `OrderLimit` fields to the `PieceDownloadResponse` protocol buffer for returning hashes.

```message PieceDownloadResponse {
    // Chunk response for download request
    message Chunk {
        int64 offset = 1;
        bytes data = 2;
    }
    Chunk chunk = 1;
    optional PieceHash hash = 2;
    optional OrderLimit limit = 3;
}
```

2. Alter the storage node code to populate `PieceHash` and `OrderLimit` when the `OrderLimit` `Action` is `GET_REPAIR`.

3. Write code which can validate a piece using a `PieceHash` and `OrderLimit` as described in design.

4. Write code which ensures that enough of the pieces downloaded by satellite share the same `PiecePublicKey` as described in design.

5. Implement custom repair download code which uses a Reed-Solomon minimum number of pieces, persisting the downloaded pieces only in memory. If statistics are available, prefer to download from faster storage nodes using the power of two choices.

6. Wire in validation logic to repair loop.  If a piece fails these checks, download another.

7. If any pieces failed any of the above checks, update the pointer in metainfo, removing those pieces.

7. Reassemble the segment using Reed-Solomon decoding and continue with repair as previously implemented.

## Open issues

We may want to get rid of distinguishing between repair_gets and gets... should we always return them on a full piece download?

64MiB is current default max segment size, it doesn't have to be equal or close to 64MiB.  Should we have an option to persist to disk?

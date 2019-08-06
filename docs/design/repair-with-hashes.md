# Repair using Piece Hashes

## Abstract

The satellite should repair files also using piece hashes to minimize CPU and bandwidth load.

## Background

The white-paper states:

> Data repair is an ongoing, costly operation that will use significant bandwidth, memory, and processing power, often impacting a single operator. As a result, repair resource usage should be aggressively minimized as much as possible.
> For repairing a segment to be effective at minimizing bandwidth usage, only as few pieces as needed for reconstruction should be downloaded. Unfortunately, Reed-Solomon is insufficient on its own for correcting errors when only a few redundant pieces are provided. Instead, piece hashes provide a better way to be confident that we’re repairing the data correctly.
> To solve this problem, hashes of every piece will be stored alongside each piece on each storage node. A validation hash that the set of hashes is correct will be stored in the pointer. During repair, the hashes of every piece can be retrieved and validated for correctness against the pointer, thus allowing each piece to be validated in its entirety. This allows the repair system to correctly assess whether or not repair has been completed successfully without using extra redundancy for the same task.

## Design

Currently, each piece is uploaded to a storage node and validated according to its SHA256 hash. The uplink-signed `uplink_piece_hash` is already stored in the `pieceinfo_` table as a serialized protocol buffer and includes the `PieceID`, `Hash`, and `Signature`. Future work includes sending this uplink-signed hash to the satellite along with each piece during repair.

Only them minimum number of pieces should be used for Reed-Solomon decoding. The satellite should check the hash and signature of each piece it downloads. `CertDB` is already be available on the Satellite for obtaining uplink public keys to validate signatures. If a piece fails the hash check, a different piece should be downloaded. Hash check failures may indicate cheating. Reporting and handling cheaters is outside the scope of this document.

## Rationale

Both download and Reed Solomon decoding create more load for a satellite than hashing, so hashing should be preferred to each.  

## Implementation

1. Add an optional `PieceHash` field to the `PieceDownloadResponse` protocol buffer for returning hashes.

2. Alter the storage node code to populate `PieceHash` when responding to the owning satellite downloads.

3. Implement changes to `SegmentRepairer.Repair()`, `ecClient.Repair()`, `decodedRanger.Range()` and any other methods to allow hash checking.

4. Implement changes to `ecClient.Get()` and any other methods to allow a minimum number of nodes to optionally be used, rather than the complete set of nodes in an OrderLimit.

5. Implement changes to `SegmentRepairer.Repair()` and any other methods to retry with another node when a single download fails, ensuring that previously downloaded pieces can be reused without downloading again.

## Open issues

Instead of implementation #3, can we somehow build a SHARanger type instead, which somehow aligns to Pieces?

Are extending the existing methods ala `decodedRanger` the best option for repair?

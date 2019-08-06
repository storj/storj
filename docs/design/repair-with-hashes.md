# Repair using Piece Hashes

## Abstract

The satellite should repair files also using piece hashes to minimize CPU and bandwidth load.

## Background

Piece hashes Data repair is an ongoing, costly operation that will use significant bandwidth, memory, and processing power, often impacting a single operator. As a result, repair resource usage should be aggressively minimized as much as possible.

For repairing a segment to be eective at minimizing bandwidth usage, only as few pieces as needed for reconstruction should be downloaded. Unfortunately, Reed-Solomon is insucient on its own for correcting errors when only a few redundant pieces are provided. Instead, piece hashes provide a better way to be confident that we’re repairing the data correctly.

To solve this problem, hashes of every piece will be stored alongside each piece on each storage node. A validation hash that the set of hashes is correct will be stored in the pointer. During repair, the hashes of every piece can be retrieved and validated for correctness against the pointer, thus allowing each piece to be validated in its entirety. This allows the repair system to correctly assess whether or not repair has been completed successfully without using extra redundancy for the same task.

## Design

Each piece is currently uploaded to a storage node and validated according to its SHA256 hash.  The uplink-signed `uplink_piece_hash` is stored in the `pieceinfo_` table as a serialized protocol buffer and includes the `PieceID`, `Hash`, and `Signature`.  This uplink-signed hash will be provided to the satellite along with each piece during repair.

The satellite should check the hash and signature of each piece it downloads. If a piece fails the hash check, a different piece should be downloaded. Only them minimum number of pieces should be used for Reed-Solomon decoding.

he uplink-signed hash should determine if anyone has attempted to cheat the system in any way.  Reporting and handling cheaters is outside the scope of this document.





## Rationale

Both download and Reed Solomon decoding create more load for a satellite than hashing, so hashing should be preferred to each.  

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)

[A discussion of issues relating to this proposal for which the author does not
know the solution. This section may be omitted if there are none.]
21 satellite/repair/repairer/repairer.go 
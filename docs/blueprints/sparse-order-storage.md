# Sparse Order Storage

This blueprint describes a way to cut down on database traffic due to bandwidth
measurement and tracking enormously. It cuts out all measurement on uploads
entirely, and reduces used serial storage to a single hash only during order
submission.

## Uploads

We don't charge or pay for uploads! Uploads cost and pay out $0. So let's simply
stop tracking bandwidth usage for uploads.

Plan:

 1. make new endpoints on storage nodes and Satellites that do uploads without
    orders.
 2. Once enough storage nodes have this endpoint, set the Satellite minimum upload
    version setting to only select nodes with a new enough version that have this
    endpoint.
 3. Rework Uplinks to only use these new endpoints.

## Order Limit creation

The only reason we need to talk to the database currently when creating order
limits is to store which bucket the order limit belongs to. Otherwise, we sign
the order limit in a way where the Satellite can validate that it signed the
order limit later. So, instead, we should encrypt the bucket id in the order
limit.

For future proofing we should make a Satellite-eyes-only generic metadata
envelope (with support for future key/value pairs) in the order limit.

We should have the Satellite keep a set of shared symmetric keys, and we always
support decrypting with any key we know of, but we only encrypt with currently
rotating-in approved keys. Keys should have an id so the envelope can identify
which key it was encrypted with.

Authenticated encryption could avoid keys needing ids.

## Order submission

The key idea is described in this
[Revocation Transparency](https://www.links.org/files/RevocationTransparency.pdf)
doc. This plan results in no double spends using no database reads!

The basic idea is a "Sparse Merkle Tree", where we essentially make a Merkle Tree
with a leaf for every possible serial number. E.g., if serial numbers are 64 bits
wide, then the Merkle tree is 64 layers deep, with 2^64 leafs. The values at the
leaves are 1 or 0, indicating if the serial number has been used. Most subtrees
in the Merkle tree will be full of zeroes.

The Satellite will keep Order windows that rotate at defined intervals (daily?).
Each Order Limit will specify which window the serial number belongs to, and
each node will keep track of its own sparse Merkle tree for that window. Windows
have a defined expiration time for new submissions based on the window id, but
the window state per node will be kept in the Satellite database instead of the
bandwidth rollups table. The Satellite will only keep a Merkle root per node
per window.

When a Storage Node wants to submit Orders, it will submit:
 1. The signed output of the previous state of that node's window from the
    Satellite.
 2. The leaves it wants to change.
 3. The subtree roots in the existing window's Sparse Merkle Tree that are needed
    to compute the root before and after the changed leaves. Notably, these
    subtree roots for subtrees that are all zeroes are well defined constants,
    of which there are only 64 in a depth-64 Merkle tree.

The Satellite will first validate the given signed state output, pontentially
overwriting the one is has (no database reads required for order submission).
Then it will recompute the Merkle root and match it with its stored one to
confirm the Storage Node isn't lying *and* hasn't submitted any of the provided
serial numbers before. Then it will update the leaves with the provided serial
numbers the Storage Node wants to submit, and create a new Merkle root. Then it
will sign the new state (which includes the Merkle root and the new bandwidth
totals for that window), store it, and submit the signed state to the storage
node.

The storage node will store that signed state for the next batch order submission
for that window.
Bandwidth rollups are as simple as querying the bandwidth totals out of the signed
state structures per node per window.

## Other concerns

### Usage limits

We think usage limits are an orthogonal problem and should be solved with a
separate system to keep track of per-bandwidth-window allocations and usage.


## References

 * https://www.links.org/files/RevocationTransparency.pdf
 * https://ethresear.ch/t/optimizing-sparse-merkle-trees/3751
 * https://eprint.iacr.org/2018/955.pdf

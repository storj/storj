# Sparse Order Storage Take 2

This blueprint describes a way to cut down on database traffic due to bandwidth
measurement and tracking enormously. It reduces used serial storage from
terabytes to a single boolean per node per hour.

## Background

Storj needs to measure bandwidth usage in a secure, cryptographic, and
correctly incentivized way to be able to pay storage node operators for the
bandwidth that the system uses from them.

Currently we use a system described in section 4.17 of the Storj V3 whitepaper,
though "bandwidth allocations" have become named "Order Limits" and
"restricted bandwidth allocations" have become named "Orders."

We track Order Limit serial numbers in the database, created at the time a
segment begins upload, and then we use that database for preventing storage
nodes from submitting the same Order more than once.

The current naive system of keeping track of serial numbers, used or unused, is
a massive amount of load on our central database, to the point that it accounts
currently for 99% of all of the data in the Satellite DB. The plan below seeks
to eliminate terabytes of data storage in used serial tracking in Postgres for
a bit per node per hour (we have a couple thousand nodes total right now).

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

We should use authenticated encryption no matter what, but authenticated
encryption could additionally avoid keys needing ids (we would have to cycle
through and try keys until one worked).

## Order submission

The main problem we're trying to solve is the concept of double spends. We want
to make sure that a Satellite and an Uplink and a Storage Node all agree on
an amount of bandwidth getting used. The way we do this as described in whitepaper
section 4.17 is to have the Satellite and Uplink work together to sign what they
intend to use, and then the Storage Node is bound to not submit more than that.
We want the Storage Node to submit the bandwidth it has used, but once it "claims"
bandwidth from an Uplink, we need to make sure it can't claim the same bandwidth
twice.

Originally, this section described a rather ingenious plan that involved reusing
tools from the efficient certificate revocation literature (sparse merkle trees),
but then we realized that we don't need any of it. Here's the new plan:

The Satellite will keep Order windows, one per hour. Each Order Limit specifies
the time it was issued, and the satellite will keep a flag per node per window
representing whether or not Orders for that window were submitted.

Storage Nodes will keep track of Orders by creation hour. Storage Nodes
currently reject Orders that are older than a couple of hours. We will make sure
this is exactly an hour, and therefore, Storage Nodes will have the property
that once an hour has expired, no new orders for that hour will come in.

As pointed out by Pentium100 on the forum: we’ll need to specifically make sure
we consider that requests must start within the hour, and then not submit orders
for a window until all of the requests that started within that hour finish.

Storage Nodes will then have 48 hours to submit their Orders, in an hour batch
at a time. Submitted Orders for an hour is an all-or-nothing process. The
Storage Node will stream to the Satellite all of its Orders for that hour, and
the Satellite will result in one of three outcomes: the orders for that
window are accepted, the window was already submitted for, or an unexpected
error occurred and the storage node should try again.

The Satellite will keep track, per window per node, whether or not it has
accepted orders for that hour. If the window has already had a submission, no
further submissions for that hour can be made.

When orders are submitted, the Satellite will update the bandwidth rollup
tables in the usual way.

## Other options

See the previous version of this document on
https://review.dev.storj.io/c/storj/storj/+/1732/3 (patchset 3)

## Other concerns

## Can storage nodes submit window batches that make the Satellite consume large amounts of memory?

Depending on the storage node, the number of orders submitted within an hour
may be substantial. Care will need to be taken about the order streaming protocol
to make sure the Satellite is able to appropriately stage its own internal updates
so that it does not try and store all of the orders in memory while processing.

### Usage limits

We think usage limits are an orthogonal problem and should be solved with a
separate system to keep track of per-bandwidth-window allocations and usage.

## References

 * https://review.dev.storj.io/c/storj/storj/+/1732/3
 * https://www.links.org/files/RevocationTransparency.pdf
 * https://ethresear.ch/t/optimizing-sparse-merkle-trees/3751
 * https://eprint.iacr.org/2018/955.pdf
 * https://www.cs.purdue.edu/homes/ninghui/papers/accumulator_acns07.pdf
 * https://en.wikipedia.org/wiki/Benaloh_cryptosystem
 * https://en.wikipedia.org/wiki/Paillier_cryptosystem#Electronic_voting
 * https://en.wikipedia.org/wiki/Homomorphic_encryption#Partially_homomorphic_cryptosystems
 * http://kodu.ut.ee/~lipmaa/papers/lip12b/cl-accum.pdf
 * https://blog.goodaudience.com/deep-dive-on-rsa-accumulators-230bc84144d9

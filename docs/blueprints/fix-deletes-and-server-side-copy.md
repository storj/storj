# Fix deletes (and server side copy!)

## Abstract

Hey, let's make deletes faster by relying on GC. If we do this, there are some
additional fun implications.

## Background/context

We are having a lot of trouble with deletes with customers. In the last month
we have received critical feedback from a couple of customers (ask if you want
to know) about how hard it is to delete a bucket. A customer wants to stop
paying us for a bucket they no longer want, maybe due to the high per-segment
fee or otherwise.

The main thing customers want is to be able to issue a delete and have us
manage the delete process in the background.

There are two kinds of deletes right now (besides setting a TTL on objects) - explicit deletes and garbage
collection. Explicit deletes are supposed to happen immediately and not result
in unpaid data for the storage node (though they don't right now), and garbage
is generated due to long tail cancelation or other reasons, but is unfortunately
a cost to storage node operators in that they are not paid for data that is
considered garbage. Garbage is cleaned up by a garbage collection process that
stores data for an additional week after being identified as garbage in the
trash for recovery purposes. We have long desired to have as many deletes be
explicit deletes as possible for the above reasons.

The way explict deletes work right now is that the Uplink sends the Satellite a
delete request. The Satellite, in an attempt to both provide backpressure and
reduce garbage, then issues delete requests to the storage nodes, while keeping
the Uplink waiting. The benefit of the Satellite doing this is that the
Satellite attempts to batch some of these delete requests.

Unfortunately, because backups are snapshots at points in time, and Satellites
might be recovered from backup, storage nodes are currently unable to fully
delete these explicitly deleted objects. The process for recovering a Satellite
from backup is to first recover its backed up metadata, and then to issue a
restore-from-trash to all storage nodes. So, as a result, any of the gains we've
tried to get from explicit deletes are illusory because explicitly deleted data
goes into the trash just like any other garbage.

It has been our intention to eventually restore the functionality of storage
nodes being able to explicitly delete data through some sort of proof-of-delete
system that storage nodes can present to amnesiatic Satellites, or to improve
the Satellite backup system to have a write ahead log so that backups don't
forget anything. But, this has remained a low priority for years, and the
costs of doing so might outweigh the benefits.

One additional consideration about explicit deletes is that it complicates
server-side copy. Server-side copy must keep track of reference counting or
reference lists so that explicit deletes are not errantly issued too soon.
Keeping track of reference counting or reference lists is a significant burden
of bookkeeping. It adds many additional corner cases in nearly every object
interaction path, and reduces the overall performance of copied objects by
increasing the amount of database requests for them.

Consider instead another option! We don't do any of this!

## Design and implementation

No explicit deletes. When an uplink deletes data, it deletes it from the
Satellite only.

The Satellite will clean the data up on the storage nodes through the standard
garbage collection process.

That's it!

In case you're wondering, here are stats about optimal bloom filter sizing:

```
pieces    size (10% false positives)
100000     58.4 KiB
1000000   583.9 KiB
10000000    5.7 MiB
100000000  57.0 MiB
```

### BUT WAIT, THERE'S MORE

If we no longer have explicit deletes, we can dramatically simplify server-side
copy! Instead of having many other tables with backreferences and keeping track
of copied objects separately and differently from uncopied objects and ancestor
objects and so on, we don't need any of that.

Copied objects can simply be full copies of the metadata, and we don't need to
keep track of when the last copy of a specific stream disappears.

This would considerably improve Satellite performance, load, and overhead on
copied objects.

This would considerably reduce the complexity of the Satellite codebase and data
model, which itself would reduce the challenges developers face when interacting
with our object model.

## Other options

Stick with the current plan.

## Migration

Migration can happen in the following order:

 * We will first need to stop doing explicit deletes everywhere, so that
   we don't accidentally delete anything.
 * Then we will need to remove the server side copy code and just make object
   copies actually just copy the straight metadata without all the copied object
   bookkeeping.
 * Once there is no risk and there is no incoming queue, then we can have a job
   that iterates through all existing copied objects and denormalizes them to
   get rid of the copied object bookkeeping.

## Wrapup

We should just do this. It feels painful to give up on explicit deletes but
considering we have not had them actually working for years and everyone seems
happy and it hasn't been any priority to fix, we could bite the bullet, commit
to this, and dramatically improve lots of other things.

It also feels painful to give up on the existing server-side copy design, but
that is a sunk cost.

## Additional Notes

1. With this proposal Storagenodes will store for more time (Until GC cleans up the files). I think it should be acceptable:

 * For objects stored for longer period time, it doesn't give big difference (1 year vs 1 year + 1 day...)
 * For object uploaded / downloaded in short period of time: It doesn't make sense just to upload + delete. For upload + download + delete, it's a good business anyway, as the big money is in egress, not in the storage. As an SNO, I am fine with this.

2. GDPR includes 'right to be forgotten'. I think this proposal should be compatible (but IANAL): if metadata (including the encryption key) is not available any more, there isn't any way to read it.

3. There is one exception: let's say I started to download some data, but meantime the owner deleted it. Explicit delete may block the read (pieces are disappearing, remaining segments might be missing...)

While this proposal would enable to finish the downloads if I already have the orderlimits from the satellite (pieces will remain there until next GC).

Don't know if this difference matters or not.

One other point on objects that are stored for a short amount of time above - we can potentially introduce a minimum storage duration to help cover costs.

## Q&A

> 1. what with node tallies? without additional bookkeeping it may be hard to not pay SNO for copies, SNO will be payed for storing single piece multiple times because we are just collecting pieces from segments to calc nodes tally.

> 2. how we will handle repairs? will we leave it as is and copy and original will be repaired on its own?

> 3. do we plan to pay for one week of additional storage? data won't be in trash.

> 4. we need to remember that currently segment copy doesn't keep pieces. pieces are main size factor for segments table. We need to take into account that if we will have duplications table size will grow. not a blocker but worth to remember.

These are good questions!

Ultimately, I think these are maybe questions for the product team to figure out, but my gut reaction is:

 * according to the stats, there are very few copied objects. copied objects form a fraction of a percent of all data
 * so, what if we just take questions one and three together and call it a wash? we overpay nodes by paying individually for each copy, and then don't pay nodes for the additional time before GC moves the deleted object to the trash? if we go this route, it also seems fine to let repair do multiple individual repairs.

i think my opinion would change if copies became a nontrivial amount of our data of course, and this may need to be revisited.

## Related work




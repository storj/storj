# Low I/O piece storage

## Abstract

Storage nodes use a plain file storage for pieces, which result in various file system overheads depending on the
specific storage node setup. This slows down nodes and puts some popular setups at disadvantage. By reimplementing
a subset of file system features in a way that is optimized for storage node use, it should be possible to reduce the
amount of I/O operations performed during routine tasks like uploads and downloads 5- to 10-fold, making it possible to
use less resources to operate larger nodes.

## Main objectives

1. Operating a performant storage node on a number of popular setups (btrfs, thin-allocated storage, parity RAIDs,
   low-memory setups especially with NTFS, SMR drives, Storage Spaces) becomes possible.
2. Reduction in I/O operations necessary to perform routine tasks leads to improvement in time-to-first-byte and latency
   metrics on setups currently considered performant.

## Background

The cheapest way to provide storage for Storj purposes is to use hard disks, which have some important performance
characteristics:
* Reading and writing data sequentially, sector by sector, does not incur penalties and can use the full speed of a hard
  drive, often >100 MB/s.
* Reading and writing data from different parts incurs penalties each time the drive moves from one area to another.
  Penalties depend on various factors [1]:
  * whether the actuator needs to move to a different track incurs a penalty formally called _settle time_,
  * numbers of tracks to move incurs a penalty formally named _seek time_,
  * the time it takes to rotate the platter so that the right part of the track is available is formally called
    latency.

  Penalties in total may reduce effective read/write speeds by even three orders of magnitude. Penalties are usually
  smaller when a latter operation has only slightly higher sector number than the former (maybe within the same
  track, maybe on a neighbouring track). The total penalty on modern hard disk drives is between 5 and 10 ms. It is
  usually assumed modern drives can perform at most 250 random I/O operations per second, regardless of total drive
  capacity.

[1] https://web.archive.org/web/20161101210900/http://www.pcguide.com/ref/hdd/perf/perf/spec/pos_Access.htm

Some ways to avoid penalties include:
* Leveraging RAM for caches. This is limited by the available amount of RAM.
* Leveraging flash storage for read caches. This requires availability of flash storage, which induces additional
  costs and may not be possible to use if the computer a node is operating on does not have free controller ports.
* Delaying writes: by collecting a number of write operations, the operating system (through various I/O scheduler
  algorithms) and the hard disk drive (through the _Native Command Queue_ feature) can reorder them to reduce
  penalties, or avoid a performing a write operation altogether (if two write operations end up modifying the same
  sector). For example, the popular deadline scheduler sorts operations by sector number to leverage the case of two
  operations in short distance.

Additional performance constraints come from popular block device setups like thinly allocated storage which might
increase penalties, or parity RAID schemes and SMR drives, which result in significant I/O amplification
for small writes. It is therefore advantageous to design data structures stored on hard disk drives to be as small
as possible (for better cache utilization), to minimize the number of seeks, and avoid forcing writes, especially
forcing many small writes.

As of now, storage nodes store each piece as a separate file in the file system, requiring an allocation of
various file system data structures. While these data structures differ between file systems, they usually include
at least two: a directory entry (_direntry_ in ext4 parlance), and a file node (usually called _inode_). As file
systems are general-purpose, they need to provide features for various use cases, not just operate storage nodes.
Many of these features negatively impact performance of storage nodes though. For example:

* Separation of direntries and inodes is necessary to support hardlinks. Yet this separation requires an additional
  seek for each file access, and storage nodes do not use hard links.
* Journals improve recovery of file system metadata after a failure (e.g. system crash). Yet they require an
  additional seek and, depending on configuration, a forced write, for each operation that modifies metadata, while
  storage nodes can easily recover some types of lost metadata operations (e.g. if a file is deleted from trash by
  one trash file walker cycle, it will be deleted by the next one).
* Many file system operations are required to be atomic, for example file creation. This requires forced
  writes. Yet storage nodes do not require atomicity for many of them (e.g. it's rather unlikely that two pieces
  with the same ID will be uploaded at the same time).
* Some file systems or storage solutions have copy-on-write semantics which allow cheap snapshots or deduplication,
  but additionally fragment data when faced with small writes.
* Inodes store metadata information, such as user permissions, date of modification, file names, etc. This makes
  inodes larger than necessary for storage node purposes. Bigger data structures end up taking more cache space.
  For example with the current average segment size of 10.2 MB: ext4 with default inode size of 256 bytes and a
  typical direntry size (for a storage node piece) of 62 bytes will require at least 904 MB per 1 TB of pieces; NTFS
  with an inode size of 1kB will require at least 2.84 GB per 1 TB of pieces. If metadata do not fit in cache, they
  will incur additional seeks for each file access.

While some aspects of file system operation can be tuned by storage node operators (e.g. changing the journal mode),
this is not always possible, e.g. because the file system is also used for other purposes.

This document describes a different approach of improving storage node operations. By reorganizing blob storage so
that there is no one-to-one correspondence between files and pieces storage node will no longer be required to
perform many file system operations. By implementing a custom way to store metadata, the amount of cache necessary
is significantly reduced.

## Design

This is a rough draft of a design. Notes that mention known missing details or additional ideas are marked with TODO.


### Data structures

Three types of files are used: a journal file, pack files to store pieces, and a piece index file to quickly find a
piece by its ID. They are roughly equivalent to the file system journaling, data blocks and direntries/inodes.

Data structure description define some magic numbers, such as the maximum pack size. It may make sense to make them
configurable, but for simplicity the document suggests certain values to focus attention on a specific solution.


#### The journal file

The journal file is an append-only file that stores a log of all modifications to metadata. There is one journal file
per satellite. The structure of this file is a series of messages denoting performed write and compaction operations,
written in a tight binary encoding (e.g. protobuf). It is only used for recovery purposes.

The file needs to be periodically synced, e.g. once a minute.

TODO: It is probably a good idea to have some sort of CRC inserted every so often, e.g. for each 8 KiB block of
messages.

TODO: This file may actually grow pretty large, so it might be a good idea to store it in chunks of, let say, 256 MB.


#### Pack files

Pack files store pieces and their original header information in a format that resembles an append-only storage.
Assume that we want to store three pieces:

| Item | Length (bytes)                       | Contents         |
|------|--------------------------------------|------------------|
| 0    | 512                                  | Piece 0's header |
| 1    | piece 0's length padded to 512 bytes | Piece 0's data   |
| 2    | 512                                  | Piece 1's header |
| 3    | piece 1's length padded to 512 bytes | Piece 1's data   |
| 4    | 512                                  | Piece 2's header |
| 5    | piece 1's length padded to 512 bytes | Piece 2's data   |

This is essentially the current blob files concatenated and padded to 512 bytes.

We will limit the size of a pack file by putting a restriction that no piece header can start after or on the offset of
256 MiB. With the current average segment size this means approximately 726 pieces per pack file. To store more pieces
we will use multiple pack files. Each pack file is identified by a 24-bit unsigned integer.

256 MiB was chosen as a trade-off between an attempt to minimize the total number of pack files, and the risk of
losing a large number of pieces at once in case a pack file is lost. It is also a nice coincidence that with this
size and counting blocks of 512 bytes, we need exactly 4 bytes total to denote both offset and length of a single
piece: 19 bits for the offset, and 13 bits for length assuming maximum size of a piece of 4 MiB. On the other side,
this limits the size of a piece, which may be undesirable.

We will be using hole punching (FALLOC_FL_PUNCH_HOLE) to free up disk space used by deleted pieces. We will be using
file collapsing (FALLOC_FL_COLLAPSE_RANGE) to compact pack files and free up address space within a pack file. The
former is available on all modern file systems, even including NTFS. The latter only on Linux, but it might be possible to
emulate it by rewriting pack files. See the _Operations_ section below.

Hole punching also means it may not be possible to jump from one header to the next one by reading piece size from
the header, as a header may be hole-punched as well. However, as locations of pieces will be stored in the journal
file, this is not necessary even for recovery purposes.

*Append offset* is the offset of the first byte after the piece's data with the highest offset (logically last). This
will be the offset of the next stored piece. This design never stores pieces inside punched holes, though it could be
investigated in future as a replacement for file collapsing.

An _active pack file_ is the file currently selected for new uploads. There is only one active pack file at a time:
it is desirable to have as many consecutive uploads land in the same file, as this allows write coalescing across
many uploads. An active pack file is kept open regardless of whether uploads are in progress. At node start, or when
the active pack file's append offset crosses 256 MiB, a new one is chosen:
1. If there are any pack files with an append offset smaller than 128 MiB, then the one with the smallest append
   offset is chosen.
2. Otherwise, a new pack file is created.

Each time a pack file is activated, the area starting from the append offset and ending at 256 MiB is preallocated to
reduce initial fragmentation and potentially speed up future writes on some setups. The last piece will likely fall
out of this region, but for one piece out of an almost thousand, this is not a big concern.

Each time a pack file is activated, a message is written into the journal: *PackFileActivated(pack file ID, append
offset)*

As we will reuse pack files after compaction, it should be possible to keep a significant majority of pack files at
least half full. This means that for 20 TB worth of pieces, we should end up with around 75k to 150k pack files.

2^24 pack files even at 128 MiB each allows for 2.2 PB of stored data per node per satellite. As such, this should be
enough for foreseeable future.


#### Piece index

There is a single index file for each satellite. Its goal is to allow quick search for pieces by piece ID in the pack
files without having to scan the journal file. There are few possible data structures here, but one that is especially
enticing is to use a hash table data structure with 2^k buckets storing 8 KiB of data each. Each bucket contains
data about all stored pieces whose piece ID starts with a certain k-bit prefix. The bucket stores its CRC, an
individual origin timestamp in days since 2020, and following information for each piece:

| Item | Length   | Contents                                                                         |
|------|----------|----------------------------------------------------------------------------------|
| 0    | 3 bytes  | Pack file identifier                                                             |
| 1    | 19 bits  | Piece's header offset in units of 512 bytes                                      |
| 2    | 13 bits  | Piece's data length in units of 512 bytes                                        |
| 3    | 32 bytes | Piece ID                                                                         |
| 4    | 15 bits  | Upload timestamp in number of days since the bucket's origin timestamp           |
| 5    | 15 bits  | Expiration/trash timestamp in number of days since the bucket's origin timestamp |
| 6    | 1 bit    | Is file trashed? "trash bit"                                                     |

The size of this file is the key to fast operation, and hence the size of the above structure needs to be made as
small as possible. As proposed, this structure takes 43 bytes. 190 entries fit in a single bucket, with 22 bytes to
spare. Those spare bytes are used to store a CRC for each bucket, the origin timestamp, and maybe some version
number for the structures stored in the bucket.

Expiration/trash timestamp stores the expiration timestamp if the piece is not trashed yet (trash bit clear), or the
trash timestamp if it is (trash bit set). It does not make much sense to expire an already trashed piece. In case of
restoring a piece from trash, we can consult the journal file, or just assume we have lost the expiration timestamp.

Upload and expiration/trash timestamps are stored in reference to the origin timestamp for the bucket. Each time a
bucket is written, all upload and trash timestamps older than "2 weeks ago" are updated to "2 weeks ago", then a
new origin timestamp is chosen as the earliest date among all timestamps. This is to make 15 bits enough of a sliding
window: 2^15 days is 89 years, which is effectively "forever" for an expiration timestamp anyway.

Structures do not have to be sorted in any particular order, like by piece ID. After reading a bucket from storage,
linearly scanning it for a given piece ID will be fast enough.

Any given entry can be declared as unused by zeroing all fields. Identifying an empty entry can be done by checking
piece's length, as no piece can have a length of zero. Coincidentally, an initially preallocated index file consists
of zeros.

The initial number of buckets should be 8192 (for k=13 and the total size of the hashmap of 64 MiB), as this is the
minimal size to contain information on 500 GB worth of data at the current average size of piece. In case a bucket
fills up, a new piece index file should be created with a k factor increased by one. Each bucket in the original
file will sequentially be mapped to two consecutive buckets in the new file, making a hash table grow a sequential
read and write operation. There is probably no need to ever shrink the piece index, as even at 20 TB worth of pieces
it would likely only grow to 4 GiB. Besides, htrees of direntries in ext4 are never shrunk as well, which is a
precedent here.

The file should be preallocated to reduce fragmentation and hence allow fast sequential scanning. A hashmap of k=19
should be enough to store information on 20 TB worth of pieces and will require 4 GiB, making a scan at 100 MB/s
take 40 seconds. This will become our equivalent of a file walker for garbage collection and trashing.

It probably makes sense to map this file to memory, as opposed to reading/writing a bucket explicitly.

This file should be considered a database and allowed to be stored on a separate file system in case node operator
desires, in a similar way to other databases. This is the only file where small random reads and writes will be
performed. This file should be marked as no-CoW on file systems which use copy-on-write semantics by default. In
case this file is lost or damaged, it can be recreated from the journal file.

In some node setups, it may even be viable to always store it in a RAM disk, as opposed to e.g. flash storage, as
persistence of this file is not necessary (except for faster startups after a clean shutdown) and the file is
already designed to fit in RAM cache in most setups. Hence, it may be desirable to have a separate directory setting
for this file. Alternatively, the design can be changed to having a piece index as just an in-memory data structure
without disk representation, leveraging swap space in case of extremely low-memory nodes.

By storing the expiration time of pieces in the journal file and this file, we effectively no longer need to have
a separate SQLite database anymore for storing them anymore.

TODO: this file needs to also have some sort of a clean shutdown check. Might be as simple as a single bit stored
whether the file is opened, set on node's start and cleared on a clean shutdown.

TODO: to prevent malicious satellites from filling up specific buckets, the k-bit prefix may actually come from a
separate hash function on a salted piece ID.


### Operations

The procedures described in this section are simplified to show the design without writing down all necessary
details. For example, most operations should consider some sort of locking for many of their steps.

In the comparisons to the current approach the following will be assumed:
1. The ratio of the unused RAM to the amount of pieces stored is smaller than 1 GB / 1 TB. This covers many
   enthusiast and SOHO setups, but also allow for much more economical professional setups even if these setups
   could use a large amounts of memory. This assumption means we cannot count on direntries and inodes of piece
   files in the current approach to be cached in RAM, as the only case this would happen with setups discussed on the
   forum is an ext4 setup tuned specifically for Storj with small inodes.
2. The ratio of the unused RAM to the amount of pieces stored is larger than 150 MB / 1 TB, allowing the piece index
   file, as well as direntries and inodes of the pack files to be kept in RAM cache. The proposed approach would
   still be better in terms of number of seeks below this ratio, but even cheap enthusiast setups are rarely below
   this threshold.
3. No SSD caching. For the purposes of a storage node an SSD cache of a non-trivial amount set up properly would bring
   the same benefits as more RAM, but it is not always possible to add an SSD to an existing setup. Besides, even
   with SSD cache, the described approach reduces the number of writes performed, prolonging its lifespan and
   allowing a cheaper consumer devices to perform.


#### Start

Run the following steps for each satellite on node start:

1. Create an index file if it does not exist. Preallocate it for k=13. Or, recover it from the journal file by
   replaying all journal messages in case of damage detected/unclean shutdown.
2. Perform the used space file walker by reading the physical sizes of all pack files, the journal file, and the
   piece index file.
3. Scan the index file. For each pack file, identify its append offset by finding the max(Piece's header offset +
   Piece's data length).
4. Open the journal file for appending.
5. Identify the active pack file. Open it for appending.

Note: we are scanning the piece index file anyway, so we could sum up the piece sizes from the index instead of running
the file walker. However, this will not account for misaligned hole punching (see the _Single immediate piece
deletion_ section notes), will not account for metadata (though the current file walker doesn't do it either), and
will not be accurate in case of an unclean shutdown. However, summing up pack files will still be 2 orders of
magnitude faster than the current used space file walker, as the number of inodes will be much smaller.


#### Upload

We will assume that we are not writing a piece to disk until we have all piece contents. This means no temporary
files and the need to store partial piece data in memory (or swap). In such case, swap replaces the role of a
temporary file, at likely lower efficiency though. It should be rare to have more than few hundreds of pieces uploaded
at a time, and even for one thousand of concurrently uploaded pieces this means memory usage of less than 3 GB. We
will consider this a fixed overhead, not dependent on the amount of data stored. It is advisable to set the limit of
concurrent uploads accordingly though.

1. Collect all data to store the piece.
2. Append the piece header and piece contents to the active pack file, padding to 512 bytes.
3. Add an entry to the piece index file. If the hash table's bucket is full, rebuild the hash table with a bigger k.
4. Add an entry to the journal: *NewPiece(upload timestamp, piece ID, piece data length, piece expiration time)*
5. Update the pack file's append offset.

Active pack file and the journal are already opened, so no need to perform additional I/O to locate them up. The
former is also preallocated. None of the writes need to be forced. As such, in most cases, pack file and journal
file writes will be coalesced across many uploads. The piece index modification will turn into a random write, but
assuming that many such writes will be collected in a short period, these writes being close to each other (in a
single preallocated file) still have a chance to be quite a bit faster than random writes across the whole drive
thanks to I/O schedulers.

This compares well to the expected 10-20 I/O operations, many of them forced writes, with the current approach.

In the event of a crash, as only the active pack file is modified, only pieces from the active pack file are likely
at risk. Given that thousands of pack files are expected, this leads to risk well below the assumed 2% of files lost.


#### Download

1. Look up the piece's location through the piece index.
2. Verify that its trash bit is not set and expiration date is not in the past.
3. Read contents of the piece from the pack file denoted in the piece index using offsets given in the piece index.

We perform one cached random read from the piece index, then a cached direntry and inode reads for the pack file. At
the end, we perform a sequential read of the requested contents. Note that despite that the pack file may
technically end up fragmented due to hole punching and compaction, fragmentation will likely occur only at the
boundaries of pieces, making it not a factor regarding single piece reads. However, this fragmentation may
affect the complexity of the pack file's extent tree. Extend trees will likely stay cached for downloads of recently
uploaded pieces (a common case), but not for older pieces.

This compares favorably to the current case, where the direntry, inode, and data from the piece itself needs to be
read from three different locations. Going by the assumption that the direntry and inode of piece files in the
current scheme are not cached, this means a potential improvement in the latency for piece data reads of 10-20 ms.


### Single immediate piece deletion

1. Look up the piece's location through the piece index.
2. Punch a hole in the pack file denoted in the piece index using offsets given in the piece index. Punching a hole
   frees up data sectors without invalidating offsets of valid pieces within the pack file.
3. Remove the piece's entry in the piece index.

We perform one cached random read from the piece index. Hole punching basically requires a change to the extents
tree of a file and a modification of the file's inode, which is two reads, two writes. As the last step there is
a write to the piece index, which does not have to be forced (worst case, a potential attempt to read from a hole).
This is roughly comparable to a file deletion, which requires removal of a direntry and an inode (at least two reads,
two writes). Both cases will also need to free up allocated data sectors.

TODO: Punching a hole requires file ranges to fall at multiplies of file system's logical block sizes. This may
mean that punching will not be effective for small pieces, and require adjusting the ranges (instead of "X to Y", we
may need to do "X+𝛿 to Y-𝜀"). It is not a big problem though: these leftovers will be at most two data blocks, and
will be properly collected by a future compaction run anyway. Also, in case of trash collection described later,
multiple hole punches next to each other can be merged into one, which may lessen the impact of this limitation.

It is not necessary to add a journal message. Worst case, the journal will recover a metadata entry for a piece whose
contents are no longer stored. Any attempt to download such piece will fail, and the metadata entry will be
garbage-collected at the next opportunity.


### Garbage collection

Scan the piece index, searching for all pieces whose Piece ID matches the bloom filter and upload timestamp is early
enough. For the pieces that do match, set their trash bit and overwrite the piece expiration/trash timestamp.

Note how this operation only touches the piece index file, and does that roughly sequentially. Even if there's just
a bunch of buckets being modified all over the hash map, any decent I/O scheduler will order writes favorably to the
hard disk layout, as we modify buckets in order. As such, we may end up in GC runs taking tens of seconds per TB
worth of pieces.

There's no point in listing the I/O necessary for the current approach.

It is not necessary to add a journal message. Worst case, the journal will recover a metadata entry for a piece whose
contents are no longer stored. Any attempt to download such piece will fail, and the metadata entry will be
garbage-collected at the next opportunity.


### Restore from trash

1. Look up the piece's location through the piece index.
2. Clear the trash bit in the piece index, and clear the expiration/trash timestamp.

As we do not store journal messages for trashing a piece, it is not necessary to store messages to restore either.


### Checking for existence of a piece

Look up the piece in the piece index. Verify that the trash bit is clear and piece is not expired.


### Trash collection, expired pieces collection and pack file compaction

1. Scan the piece index, searching for all pieces whose trash bit is set and expiration/trash timestamp is not old
   enough, or trash bit is clear and expiration/trash timestamp is in future (i.e., all pieces to be kept):
   1. Tally up their total size (in terms of the number of file system's logical blocks, not in terms of bytes) for
      each pack file.
   2. Note down all piece IDs not matching (i.e., to be removed) together with their pack file identifier.
2. For each pack file identified as having at least one piece to be removed:
   1. If the pack file is active, just run the procedure _Single immediate piece deletion_ for each removed piece.
       Compaction cannot be run together with uploads.
   2. If the append offset of the pack file is already below 128 MiB, just run the procedure _Single immediate
      piece deletion_ for each removed piece. No point in compacting _again_.
   3. If the total size of pieces kept is still bigger than 128 MiB, just run the procedure _Single immediate
      piece deletion_ for each removed piece. No point in compacting _yet_.
   4. Otherwise, collect the pack file ID as a candidate for compaction.
3. Scan the piece index again, this time searching for all piece IDs stored in pack files that are candidates for
   compaction. While scanning, immediately erase pieces whose trash bit is set and expiration/trash timestamp is early
   enough, or trash bit is clear and expiration/trash timestamp is in the past.
4. For each pack file that is a candidate for compaction, run compaction with the piece IDs to be kept.

Total size can be computed exactly (e.g. by using a bit vector with one bit per file system's logical block). Then even
for 20 TB of data with the popular logical block size of 4 KiB this means a bitmask of 671 MB. Or approximate, by
overcounting blocks shared by multiple pieces.

Scanning the piece index that is already fully cached requires no I/O. Doing it twice does neither.

The cases running the _Single immediate piece deletion_ procedure will be slightly cheaper than the current trash
collection implementation due to better locality of consecutive writes in the piece index file. Not much cheaper,
but still a bit more efficient.

TODO: In this case, if we happen to punch holes next to each other during consecutive _Single immediate piece deletion_
steps, these holes can be merged into one. This may help leaving less unaligned data.

The full compaction routine is only executed if amount of data left in the pack file is so small that it will again
make sense to use the pack file as an active file. Compaction is necessary, as otherwise we could end up in a corner
case where each pack file could at some point be left with just a small number of pieces, making the proposed design
require again an uncacheable amount of inodes and direntries. However, compaction is not friendly to concurrent
uploads and downloads, so we prefer to only run it when the expected results are good enough.

To compact a pack file, the following steps represent a simplified version of the procedure. This is probably the
most complex element of the design. It is also the most risky one, as a crash in the middle of the procedure may make
the pack file under compaction unusable.

1. Allocate a new pack file identifier.
2. Sort pieces kept by piece's header offset.
3. Note down all unused pack file ranges between pieces. E.g. if the first piece to be kept starts at block 15 and
   has data length of 5, and the second piece starts at block 31, then unused ranges include 0-14 and 21-30.
4. Shift offsets of all pieces as if there was no empty space between entries, taking into account any adjustments
   necessary (see notes below).
5. Write an entry into the journal file: *Compaction(new pack file identifier)*.
6. Perform FALLOC_FL_COLLAPSE_RANGE on the noted down unused pack file ranges.
7. Move the pack file to the new identifier.
8. Update piece index entries with a new pack file identifier and new offsets.
9. Write entries into the journal file for each piece: *CompactedPiece(piece ID, new offset)*.
10. Write an entry into the journal file: *PackFileRemoved(old pack file identifier)*.
11. Update the append offset for the pack file, so that it can be considered ready to become active in future.

We take advantage of the fact that the piece index is cached: despite that we will effectively perform reads and
writes in the piece index for each existing file (as opposed to doing operations only for removed files), these will
all be cached reads and localized writes. As such, the only I/O operations done are journal writes,
FALLOC_FL_COLLAPSE_RANGE, and a single file move.

Notes/TODO:
1. FALLOC_FL_COLLAPSE_RANGE requires file ranges to fall at multiples of file system's logical block sizes. As such,
   we may need to detect the exact multiplies allowed, and adjust the ranges (instead of "X to Y", we may need to do
   "X+𝛿 to Y-𝜀", with care taken that small ranges may collapse to an empty set).
2. Rounding up piece sizes to file system's logical block sizes is necessary to make sure that after the adjustment
   above we will still end up with a pack file below 128 MiB.
3. Concurrent downloads that do not read from the currently compacted file are fine. Concurrent downloads that would
   read from the compacted file cannot operate concurrently with compaction steps 6-8. These steps will most
   probably take seconds, as FALLOC_FL_COLLAPSE_RANGE is a pretty fast operation (an update to the extents tree plus
   freeing up data sectors).
4. In absence of the FALLOC_FL_COLLAPSE_RANGE operation it might actually be fine to just rewrite the pack file.
   This is a roughly sequential read and a sequential write of at most 128 MiB, which should finish in few seconds.
   As the compaction procedure of a pack file is only run at most once per 128 MiB of uploads, it may still be a
   good trade-off.
5. The PackFileRemoved message is necessary to avoid accidental overlap in terms of file range of a stored piece
   with a recovered outdated entry of an already removed file. Hole-punching that outdated entry may prove disastrous.


### Journal rewrite

A new periodic operation is performed: a journal is rewritten to get rid of outdated items, such as files removed,
or old compaction events. The procedure is performed only if journal grows twice as big as the piece index
file.

1. A new journal file is created.
2. The first message is: *PieceIndexSize(k)*, so that in case of recovery, the right size of a piece index is known
   already at the beginning of the recovery procedure.
3. All entries from the index file as written down into the new file as a sequence of messages: *PresentPiece(…)*,
   copying all data from the piece index except for the trash bit, and the expiration/trash timestamp if the trash
   bit was set.
4. fsync(), rename over the old journal file.

This is a sequential write roughly the size of the piece index file.

To allow concurrent operations, for the period of time this procedure runs, new journal messages need to be enqueued
for both old and new journal.

It might make sense to also perform this operation on a clean shutdown.

Trash status cannot be stored, as the journal does not store messages for restore from trash procedure. This is
not a concern though, the next garbage collection will deal with these pieces anyway.


## Rationale

The suggested design is proposed as an alternative to the existing piece storage. The design is more complex and
requires a careful implementation, especially regarding concurrency, recovery in case of problems. It depends on a
specific file system features like hole punching and file collapsing, which are not always available. It also
puts bigger reliability requirements towards the operating system's implementation of file system. Yet for a
significant majority of existing storage nodes it may bring a significant performance improvement and reduce wear.
It may also improve system-level metrics like time to first byte by a non-trivial amount if most nodes can adopt
this design.


## Implementation

…

## Wrapup

…

## Open issues

Migration from the existing file-based structure.

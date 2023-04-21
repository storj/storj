// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package gc contains the functions needed to run garbage collection.

Package gc/bloomfilter creates retain instructions. These are compressed
lists of piece id's that storage nodes must keep. It uploads those to a bucket.

gc/bloomfilter is designed to run on a satellite pointed to a backup snapshot
of the metainfo database. It should not be run on a live satellite: due to the
features move and copy object, pieces can move to different segments in the
segments table during the iteration of that table. Because of that it is not
guaranteed that all piece ids are in the bloom filter if the segments table
is live.

Package gc/sender reads the retain instructions from the bucket and sends
them to the storage nodes. It is intended to run on a satellite connected
to the live database.

Should we ever delete all segments from the satellite's metainfo, then no
bloom filters will be generated, because GC only considers NodeID's inside
the segments table. There is also an explicit check that stops sending out
an empty bloom filter to a storage node.

See storj/docs/blueprints/garbage-collection.md for more info.
*/
package gc

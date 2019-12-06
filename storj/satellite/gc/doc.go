// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package gc contains the functions needed to run garbage collection.

The gc.PieceTracker implements the metainfo loop Observer interface
allowing us to subscribe to the loop to get information for every segment
in the metainfo database.

The gc.PieceTracker handling functions are used by the gc.Service to periodically
account for all existing pieces on storage nodes and create "retain requests"
which contain a bloom filter of all pieces that possibly exist on a storage node.

The gc.Service will send that request to the storagenode after a full metaloop
iteration, and the storage node will use that request to delete the "garbage" pieces
that are not in the bloom filter.

See storj/docs/design/garbage-collection.md for more info.
*/
package gc

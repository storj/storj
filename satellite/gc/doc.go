// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package gc contains the functions needed to run garbage collection.

The data repair checker uses the gc.Service to periodically account for all
existing pieces on storage nodes and create "retain requests" which contain
a bloom filter of all pieces that possibly exist on a storage node.
The storage node will receive that request, and delete the "garbage" pieces
that are not in the bloom filter.

At the end of a loop, the checker will use gc.Service.Send to send out
retain requests to all storage nodes.

The piece tracker accumulates all of the bloom filters for the storage nodes
by saving them in a map of RetainInfos which are used to make RetainRequests.

When it's not time for a garbage collection run, piece tracker will be set to nil
and no RetainInfos will be saved.

See storj/docs/design/garbage-collection.md for more info.
*/
package gc

// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package bloomfilter contains the functions needed to run part of garbage collection
process.

The bloomfilter.PieceTracker implements the segments loop Observer interface
allowing us to subscribe to the loop to get information for every segment
in the metabase db.

The bloomfilter.PieceTracker handling functions are used by the bloomfilter.Service
to periodically account for all existing pieces on storage nodes and create
"retain requests" which contain a bloom filter of all pieces that possibly exist
on a storage node.

The bloomfilter.Service will send that requests to the Storj bucket after a full
segments loop iteration. After that bloom filters will be downloaded and sent
to the storage nodes with separate service from storj/satellite/gc package.

This bloom filter service should be run only against immutable database snapshot.

See storj/docs/design/garbage-collection.md for more info.
*/
package bloomfilter

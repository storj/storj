// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package bloomfilter contains the functions needed to run part of garbage collection
process.

The bloomfilter.Observer implements the ranged loop Observer interface
allowing us to subscribe to the loop to get information for every segment
in the metabase db.

The bloomfilter.Observer is subscribed to ranged loop instance to account for all
existing segment pieces on storage nodes and create "retain requests" which contain
a bloom filter of all pieces that possibly exist on a storage node. With ranged loop
segments can be processed in parallel to speed up process.

The bloomfilter.Observer will send that requests to the Storj bucket after a full
ranged loop iteration. After that bloom filters will be downloaded and sent
to the storage nodes with separate service from storj/satellite/gc/sender package.

This bloom filter service should be run only against immutable database snapshot.

See storj/docs/design/garbage-collection.md for more info.
*/
package bloomfilter

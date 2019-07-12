// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package gc contains the functions needed to run garbage collection.

The Service implementation in satellite/gc/service.go allows the satellite to send
retain requests to storage nodes. Piece retain requests contain bloom filter, that
contain possibly existing pieces. The storage node will check if it has any pieces
that are not in the retain request, and delete those "garbage" pieces.

The piece tracker implementation in satellite/gc/piecetracker.go accumulates
bloom filters.
*/
package gc

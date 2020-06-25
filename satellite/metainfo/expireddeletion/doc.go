// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package expireddeletion contains the functions needed to run expired segment deletion

The expireddeletion.expiredDeleter implements the metainfo loop Observer interface
allowing us to subscribe to the loop to get information for every segment
in the metainfo database.

The expireddeletion chore will subscribe the deleter to the metainfo loop
and delete any expired segments from metainfo.
*/
package expireddeletion

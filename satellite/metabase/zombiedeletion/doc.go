// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package zombiedeletion contains the functions needed to run zombie objects deletion chore.

The zombiedeletion chore will periodically query metabase for zombie objects
and delete them with their segments.
*/
package zombiedeletion

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package uplink provides variety of functions to access the objects using storj's
uplink library

The following functionalities are supported:

Bucket functionality

CreateBucket  - creates a new bucket if authorized
DeleteBucket  - deletes a bucket if authorized
ListBuckets   - list authorized buckets
GetBucketInfo - returns info about the requested bucket if authorized
OpenBucket    - returns a Bucket handle with the given EncryptionAccess

Object functionality

OpenObject    - returns an Object handle, if authorized
UploadObject  - uploads a new object, if authorized
DeleteObject  - removes an object, if authorized
ListObjects   - lists objects a user is authorized to see

*/
package uplink

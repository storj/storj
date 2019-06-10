// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/storj/pkg/storj"
)

func printLookup(revealed map[string]string, consumed interface{ Raw() string }, base *Base) {
	if base == nil {
		fmt.Printf("<%q, %q, nil>\n", revealed, consumed.Raw())
	} else {
		fmt.Printf("<%q, %q, <%q, %q, %q>>\n",
			revealed, consumed.Raw(), base.Unencrypted.Raw(), base.Encrypted.Raw(), base.Key[:2])
	}
}

func toKey(val string) (out storj.Key) {
	copy(out[:], val)
	return out
}

func abortIfError(err error) {
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}
}

func ubp(bucket, path string) storj.UnencryptedBucketPath {
	return storj.NewUnencryptedPath(path).WithBucket(bucket)
}

func ebp(bucket, path string) storj.EncryptedBucketPath {
	return storj.NewEncryptedPath(path).WithBucket(bucket)
}

func ep(path string) storj.EncryptedPath {
	return storj.NewEncryptedPath(path)
}

func ExampleStore() {
	s := NewStore()

	// ubp: UnencryptedBucketPath
	// ebp: EncryptedBucketPath
	// ep:  EncryptedPath

	// Add a fairly complicated tree to the store.
	abortIfError(s.Add(ubp("b1", "u1/u2/u3"), ep("e1/e2/e3"), toKey("k3")))
	abortIfError(s.Add(ubp("b1", "u1/u2/u3/u4"), ep("e1/e2/e3/e4"), toKey("k4")))
	abortIfError(s.Add(ubp("b1", "u1/u5"), ep("e1/e5"), toKey("k5")))
	abortIfError(s.Add(ubp("b1", "u6"), ep("e6"), toKey("k6")))
	abortIfError(s.Add(ubp("b1", "u6/u7/u8"), ep("e6/e7/e8"), toKey("k8")))
	abortIfError(s.Add(ubp("b2", "u1"), ep("e1"), toKey("k1")))

	// Look up some complicated queries by the unencrypted path.
	printLookup(s.LookupUnencrypted(ubp("b1", "u1")))
	printLookup(s.LookupUnencrypted(ubp("b1", "u1/u2/u3")))
	printLookup(s.LookupUnencrypted(ubp("b1", "u1/u2/u3/u6")))
	printLookup(s.LookupUnencrypted(ubp("b1", "u1/u2/u3/u4")))
	printLookup(s.LookupUnencrypted(ubp("b1", "u6/u7")))
	printLookup(s.LookupUnencrypted(ubp("b2", "u1")))

	fmt.Println()

	// Look up some complicated queries by the encrypted path.
	printLookup(s.LookupEncrypted(ebp("b1", "e1")))
	printLookup(s.LookupEncrypted(ebp("b1", "e1/e2/e3")))
	printLookup(s.LookupEncrypted(ebp("b1", "e1/e2/e3/e6")))
	printLookup(s.LookupEncrypted(ebp("b1", "e1/e2/e3/e4")))
	printLookup(s.LookupEncrypted(ebp("b1", "e6/e7")))
	printLookup(s.LookupEncrypted(ebp("b2", "e1")))

	// output:
	//
	// <map["e2":"u2" "e5":"u5"], "u1", nil>
	// <map["e4":"u4"], "u1/u2/u3", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "u1/u2/u3/", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "u1/u2/u3/u4", <"u1/u2/u3/u4", "e1/e2/e3/e4", "k4">>
	// <map["e8":"u8"], "u6/", <"u6", "e6", "k6">>
	// <map[], "u1", <"u1", "e1", "k1">>
	//
	// <map["u2":"e2" "u5":"e5"], "e1", nil>
	// <map["u4":"e4"], "e1/e2/e3", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "e1/e2/e3/", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "e1/e2/e3/e4", <"u1/u2/u3/u4", "e1/e2/e3/e4", "k4">>
	// <map["u8":"e8"], "e6/", <"u6", "e6", "k6">>
	// <map[], "e1", <"u1", "e1", "k1">>
}

func TestStoreErrors(t *testing.T) {
	s := NewStore()

	// Too many encrypted parts
	require.Error(t, s.Add(ubp("b1", "u1"), ep("e1/e2/e3"), storj.Key{}))

	// Too many unencrypted parts
	require.Error(t, s.Add(ubp("b1", "u1/u2/u3"), ep("e1"), storj.Key{}))

	// Mismatches
	require.NoError(t, s.Add(ubp("b1", "u1"), ep("e1"), storj.Key{}))
	require.Error(t, s.Add(ubp("b1", "u2"), ep("e1"), storj.Key{}))
	require.Error(t, s.Add(ubp("b1", "u1"), ep("f1"), storj.Key{}))
}

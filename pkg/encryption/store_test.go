// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storj"
)

func printLookup(revealed map[string]string, consumed interface{ Raw() string }, base *Base) {
	if base == nil {
		fmt.Printf("<%q, %q, nil>\n", revealed, consumed.Raw())
	} else {
		fmt.Printf("<%q, %q, <%q, %q, %q>>\n",
			revealed, consumed, base.Unencrypted, base.Encrypted, base.Key[:2])
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

func ExampleStore() {
	s := NewStore()
	ep := paths.NewEncrypted
	up := paths.NewUnencrypted

	// Add a fairly complicated tree to the store.
	abortIfError(s.Add("b1", up("u1/u2/u3"), ep("e1/e2/e3"), toKey("k3")))
	abortIfError(s.Add("b1", up("u1/u2/u3/u4"), ep("e1/e2/e3/e4"), toKey("k4")))
	abortIfError(s.Add("b1", up("u1/u5"), ep("e1/e5"), toKey("k5")))
	abortIfError(s.Add("b1", up("u6"), ep("e6"), toKey("k6")))
	abortIfError(s.Add("b1", up("u6/u7/u8"), ep("e6/e7/e8"), toKey("k8")))
	abortIfError(s.Add("b2", up("u1"), ep("e1'"), toKey("k1")))
	abortIfError(s.Add("b3", paths.Unencrypted{}, paths.Encrypted{}, toKey("m1")))

	// Look up some complicated queries by the unencrypted path.
	printLookup(s.LookupUnencrypted("b1", up("u1")))
	printLookup(s.LookupUnencrypted("b1", up("u1/u2/u3")))
	printLookup(s.LookupUnencrypted("b1", up("u1/u2/u3/u6")))
	printLookup(s.LookupUnencrypted("b1", up("u1/u2/u3/u4")))
	printLookup(s.LookupUnencrypted("b1", up("u6/u7")))
	printLookup(s.LookupUnencrypted("b2", up("u1")))
	printLookup(s.LookupUnencrypted("b3", paths.Unencrypted{}))
	printLookup(s.LookupUnencrypted("b3", up("z1")))

	fmt.Println()

	// Look up some complicated queries by the encrypted path.
	printLookup(s.LookupEncrypted("b1", ep("e1")))
	printLookup(s.LookupEncrypted("b1", ep("e1/e2/e3")))
	printLookup(s.LookupEncrypted("b1", ep("e1/e2/e3/e6")))
	printLookup(s.LookupEncrypted("b1", ep("e1/e2/e3/e4")))
	printLookup(s.LookupEncrypted("b1", ep("e6/e7")))
	printLookup(s.LookupEncrypted("b2", ep("e1'")))
	printLookup(s.LookupEncrypted("b3", paths.Encrypted{}))
	printLookup(s.LookupEncrypted("b3", ep("z1")))

	// output:
	//
	// <map["e2":"u2" "e5":"u5"], "u1", nil>
	// <map["e4":"u4"], "u1/u2/u3", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "u1/u2/u3/", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "u1/u2/u3/u4", <"u1/u2/u3/u4", "e1/e2/e3/e4", "k4">>
	// <map["e8":"u8"], "u6/", <"u6", "e6", "k6">>
	// <map[], "u1", <"u1", "e1'", "k1">>
	// <map[], "", <"", "", "m1">>
	// <map[], "", <"", "", "m1">>
	//
	// <map["u2":"e2" "u5":"e5"], "e1", nil>
	// <map["u4":"e4"], "e1/e2/e3", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "e1/e2/e3/", <"u1/u2/u3", "e1/e2/e3", "k3">>
	// <map[], "e1/e2/e3/e4", <"u1/u2/u3/u4", "e1/e2/e3/e4", "k4">>
	// <map["u8":"e8"], "e6/", <"u6", "e6", "k6">>
	// <map[], "e1'", <"u1", "e1'", "k1">>
	// <map[], "", <"", "", "m1">>
	// <map[], "", <"", "", "m1">>
}

func TestStoreErrors(t *testing.T) {
	s := NewStore()
	ep := paths.NewEncrypted
	up := paths.NewUnencrypted

	// Too many encrypted parts
	require.Error(t, s.Add("b1", up("u1"), ep("e1/e2/e3"), storj.Key{}))

	// Too many unencrypted parts
	require.Error(t, s.Add("b1", up("u1/u2/u3"), ep("e1"), storj.Key{}))

	// Mismatches
	require.NoError(t, s.Add("b1", up("u1"), ep("e1"), storj.Key{}))
	require.Error(t, s.Add("b1", up("u2"), ep("e1"), storj.Key{}))
	require.Error(t, s.Add("b1", up("u1"), ep("f1"), storj.Key{}))
}

func TestStoreErrorState(t *testing.T) {
	s := NewStore()
	ep := paths.NewEncrypted
	up := paths.NewUnencrypted

	// Do an empty lookup.
	revealed1, consumed1, base1 := s.LookupUnencrypted("b1", up("u1/u2"))

	// Attempt to do an addition that fails.
	require.Error(t, s.Add("b1", up("u1/u2"), ep("e1/e2/e3"), storj.Key{}))

	// Ensure that we get the same results as before
	revealed2, consumed2, base2 := s.LookupUnencrypted("b1", up("u1/u2"))

	assert.Equal(t, revealed1, revealed2)
	assert.Equal(t, consumed1, consumed2)
	assert.Equal(t, base1, base2)
}

func TestStoreIterate(t *testing.T) {
	s := NewStore()
	ep := paths.NewEncrypted
	up := paths.NewUnencrypted

	type storeEntry struct {
		bucket string
		unenc  paths.Unencrypted
		enc    paths.Encrypted
		key    storj.Key
	}
	expected := map[storeEntry]struct{}{
		{"b1", up("u1/u2/u3"), ep("e1/e2/e3"), toKey("k3")}:         {},
		{"b1", up("u1/u2/u3/u4"), ep("e1/e2/e3/e4"), toKey("k4")}:   {},
		{"b1", up("u1/u5"), ep("e1/e5"), toKey("k5")}:               {},
		{"b1", up("u6"), ep("e6"), toKey("k6")}:                     {},
		{"b1", up("u6/u7/u8"), ep("e6/e7/e8"), toKey("k8")}:         {},
		{"b2", up("u1"), ep("e1'"), toKey("k1")}:                    {},
		{"b3", paths.Unencrypted{}, paths.Encrypted{}, toKey("m1")}: {},
	}

	for entry := range expected {
		require.NoError(t, s.Add(entry.bucket, entry.unenc, entry.enc, entry.key))
	}

	got := make(map[storeEntry]struct{})
	require.NoError(t, s.Iterate(func(bucket string, unenc paths.Unencrypted, enc paths.Encrypted, key storj.Key) error {
		got[storeEntry{bucket, unenc, enc, key}] = struct{}{}
		return nil
	}))
	require.Equal(t, expected, got)
}

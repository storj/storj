package vpath

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func printLookup(revealed map[string]string, consumed string, base *Base) {
	if base == nil {
		fmt.Printf("<%q, %q, nil>\n", revealed, consumed)
	} else {
		fmt.Printf("<%q, %q, <%q, %q>>\n",
			revealed, consumed, base.Encrypted, base.Key)
	}
}

func abortIfError(err error) {
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}
}

func ExampleSearcher() {
	s := NewSearcher()

	abortIfError(s.Add("u1/u2/u3", "e1/e2/e3", []byte("k3")))
	abortIfError(s.Add("u1/u2/u3/u4", "e1/e2/e3/e4", []byte("k4")))
	abortIfError(s.Add("u1/u5", "e1/e5", []byte("k5")))
	abortIfError(s.Add("u6", "e6", []byte("k6")))
	abortIfError(s.Add("u6/u7/u8", "e6/e7/e8", []byte("k8")))

	printLookup(s.Lookup("u1"))
	printLookup(s.Lookup("u1/u2/u3"))
	printLookup(s.Lookup("u1/u2/u3/u6"))
	printLookup(s.Lookup("u1/u2/u3/u4"))
	printLookup(s.Lookup("u6/u7"))

	// output:
	//
	// <map["e2":"u2" "e5":"u5"], "u1", nil>
	// <map["e4":"u4"], "u1/u2/u3", <"e1/e2/e3", "k3">>
	// <map[], "u1/u2/u3/", <"e1/e2/e3", "k3">>
	// <map[], "u1/u2/u3/u4", <"e1/e2/e3/e4", "k4">>
	// <map["e8":"u8"], "u6/", <"e6", "k6">>
}

func TestSearcherErrors(t *testing.T) {
	s := NewSearcher()

	// Too many encrypted parts
	require.Error(t, s.Add("u1", "e1/e2/e3", nil))

	// Too many unencrypted parts
	require.Error(t, s.Add("u1/u2/u3", "e1", nil))

	// Mismatches
	require.NoError(t, s.Add("u1", "e1", nil))
	require.Error(t, s.Add("u2", "e1", nil))
	require.Error(t, s.Add("u1", "f1", nil))
}

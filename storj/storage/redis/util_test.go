// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"bytes"
	"testing"
)

func TestEscapeMatch(t *testing.T) {
	type escaped struct{ unescaped, escaped string }
	var examples = []escaped{
		{`h?llo`, `h\?llo`},
		{`h*llo`, `h\*llo`},
		{`h[ae]llo`, `h\[ae\]llo`},
		{`h[^e]llo`, `h\[^e\]llo`},
		{`h[a-b]llo`, `h\[a-b\]llo`},
		{`h\[a-b\]llo`, `h\\\[a-b\\\]llo`},
	}

	for _, example := range examples {
		got := escapeMatch([]byte(example.unescaped))
		if !bytes.Equal(got, []byte(example.escaped)) {
			t.Errorf("fail %q got %q expected %q", example.unescaped, got, example.escaped)
		}
	}
}

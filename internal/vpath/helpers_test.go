package vpath

import (
	"testing"
)

func TestPathWalker(t *testing.T) {
	type testCase struct {
		Path  string
		Parts []string
	}

	cases := []testCase{
		{Path: "", Parts: nil},
		{Path: "u1", Parts: []string{"u1"}},
		{Path: "u1/u2", Parts: []string{"u1", "u2"}},
		{Path: "u1/u2/u3", Parts: []string{"u1", "u2", "u3"}},
	}

	for _, test := range cases {
		path := newPathWalker(test.Path)

		for !path.Empty() {
			if exp, got := test.Parts[0], path.Next(); exp != got {
				t.Fatal("exp:", exp, "got:", got)
			}
			test.Parts = test.Parts[1:]
		}

		if len(test.Parts) != 0 {
			t.Fatal("extra parts:", test.Parts)
		}
	}
}

package memory_test

import (
	"testing"

	"storj.io/storj/internal/memory"
)

const (
	tb = 1 << 40
	gb = 1 << 30
	mb = 1 << 20
	kb = 1 << 10
)

func TestSize(t *testing.T) {
	var tests = []struct {
		size memory.Size
		text string
	}{
		// basics
		{1 * tb, "1.00TB"},
		{1 * gb, "1.00GB"},
		{1 * mb, "1.00MB"},
		{1 * kb, "1.00KB"},
		{1, "1B"},
		// complicated
		{68 * tb, "68.00TB"},
		{500, "500B"},
		{5, "5B"},
		{1, "1B"},
		{0, "0"},
	}

	for i, test := range tests {
		if test.size.String() != test.text {
			t.Errorf("%d. invalid text got %v expected %v", i, test.size.String(), test.text)
		}

		var size memory.Size
		err := size.Set(test.text)
		if err != nil {
			t.Errorf("%d. got error %v", i, err)
		}
		if test.size != size {
			t.Errorf("%d. invalid size got %d expected %d", i, size, test.size)
		}
	}
}

func TestParse(t *testing.T) {
	var tests = []struct {
		size memory.Size
		text string
	}{
		// case insensitivity
		{1 * tb, "1.00TB"},
		{1 * gb, "1.00gB"},
		{1 * mb, "1.00Mb"},
		{1 * kb, "1.00kb"},
		{1, "1.00"},
		// without B suffix
		{1 * tb, "1.00T"},
		{1 * gb, "1.00g"},
		{1 * mb, "1.00M"},
		{1 * kb, "1.00k"},
	}

	for i, test := range tests {
		var size memory.Size
		size.Set(test.text)
		if test.size != size {
			t.Errorf("%d. invalid size got %d expected %d", i, size, test.size)
		}
	}
}

func TestInvalidParse(t *testing.T) {
	var tests = []string{
		"1.0Q",
		"1.0QB",
		"z1.0KB",
		"z1.0Q",
		"1.0zQ",
		"1.0zQB",
	}

	for i, test := range tests {
		var size memory.Size
		err := size.Set(test)
		if err == nil {
			t.Errorf("%d. didn't get error", i)
		}
	}
}

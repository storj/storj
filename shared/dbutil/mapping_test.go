// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscapableCommaSplit(t *testing.T) {
	for _, testcase := range []struct {
		input    string
		expected []string
	}{
		{"", []string{""}},
		{",", []string{"", ""}},
		{",hello", []string{"", "hello"}},
		{"hello,", []string{"hello", ""}},
		{"hello,there", []string{"hello", "there"}},
		{"hello,,there", []string{"hello,there"}},
		{",,hello", []string{",hello"}},
		{"hello,,", []string{"hello,"}},
		{"hello,,,there", []string{"hello,", "there"}},
		{"hello,,,,there", []string{"hello,,there"}},
	} {
		require.Equal(t, testcase.expected, EscapableCommaSplit(testcase.input))
	}
}

func TestParseDBMapping(t *testing.T) {
	for _, testcase := range []struct {
		input    string
		expected map[string]string
		err      error
	}{
		{"db://host", map[string]string{"": "db://host"}, nil},
		{"db://host,override:db2://host2/db,,name",
			map[string]string{"": "db://host", "override": "db2://host2/db,name"}, nil},
		{"db://host,db2://host2", nil,
			fmt.Errorf("invalid db mapping spec: %q", "db://host,db2://host2")},
	} {
		actual, err := ParseDBMapping(testcase.input)
		if testcase.err != nil {
			require.Equal(t, testcase.err, err)
		} else {
			require.Equal(t, testcase.expected, actual)
		}
	}
}

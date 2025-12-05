// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
)

func TestParseTagPairs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		confirm       bool
		expected      []*pb.Tag
		expectedError string
	}{
		{
			name:          "comma separated tag pairs without confirm flag",
			args:          []string{"key1=value1,key2=value2"},
			expectedError: "multiple tags should be separated by spaces instead of commas, or specify --confirm to enable commas in tag values",
		},
		{
			name:    "comma separated tag pairs with confirm flag",
			args:    []string{"key1=value1,key2=value2"},
			confirm: true,
			expected: []*pb.Tag{
				{
					Name:  "key1",
					Value: []byte("value1,key2=value2"),
				},
			},
		},
		{
			name:    "single tag pair",
			args:    []string{"key1=value1"},
			confirm: true,
			expected: []*pb.Tag{
				{
					Name:  "key1",
					Value: []byte("value1"),
				},
			},
		},
		{
			name:    "multiple tag pairs",
			args:    []string{"key1=value1", "key2=value2"},
			confirm: true,
			expected: []*pb.Tag{
				{
					Name:  "key1",
					Value: []byte("value1"),
				},
				{
					Name:  "key2",
					Value: []byte("value2"),
				},
			},
		},
		{
			name:    "multiple tag pairs with comma values and confirm flag",
			args:    []string{"key1=value1", "key2=value2,value3"},
			confirm: true,
			expected: []*pb.Tag{
				{
					Name:  "key1",
					Value: []byte("value1"),
				},
				{
					Name:  "key2",
					Value: []byte("value2,value3"),
				},
			},
		},
		{
			name:          "multiple tag pairs with comma values without confirm flag",
			args:          []string{"key1=value1", "key2=value2,value3"},
			expectedError: "multiple tags should be separated by spaces instead of commas, or specify --confirm to enable commas in tag values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTagPairs(tt.args, tt.confirm)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

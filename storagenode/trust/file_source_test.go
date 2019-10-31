// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSourceNew(t *testing.T) {
	for _, tt := range []struct {
		name    string
		fileURL string
		err     string
	}{
		{
			name:    "not a valid URL",
			fileURL: "://",
			err:     `trust: invalid file source "://": not a URL: parse ://: missing protocol scheme`,
		},
		{
			name:    "not a file URL",
			fileURL: "/path",
			err:     `trust: invalid file source "/path": scheme is not supported`,
		},
		{
			name:    "invalid host",
			fileURL: "file://bad/path",
			err:     `trust: invalid file source "file://bad/path": host must be empty or "localhost"`,
		},
		{
			name:    "user info not allowed",
			fileURL: "file://john@localhost/path",
			err:     `trust: invalid file source "file://john@localhost/path": user info is not allowed`,
		},
		{
			name:    "query values not allowed",
			fileURL: "file:///path?oh=no",
			err:     `trust: invalid file source "file:///path?oh=no": query values are not allowed`,
		},
		{
			name:    "fragment not allowed",
			fileURL: "file:///path#OHNO",
			err:     `trust: invalid file source "file:///path#OHNO": fragment is not allowed`,
		},
		{
			name:    "path missing",
			fileURL: "file://",
			err:     `trust: invalid file source "file://": path is missing`,
		},
		{
			name:    "success with no host",
			fileURL: "file:///path",
		},
		{
			name:    "success with localhost",
			fileURL: "file://localhost/path",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFileSource(tt.fileURL)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestFileSourceString(t *testing.T) {
	source, err := NewFileSource("file:///path")
	require.NoError(t, err)
	require.Equal(t, "file:///path", source.String())
}

func TestFileSourceIsFixed(t *testing.T) {
	source, err := NewFileSource("file:///path")
	require.NoError(t, err)
	require.True(t, source.Fixed(), "file source is unexpectedly not fixed")
}

func TestFileSourceFetchEntries(t *testing.T) {
	url1, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@us-central-1.tardigrade.io:7777")
	require.NoError(t, err)

	url2, err := ParseSatelliteURL("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777")
	require.NoError(t, err)

	// Prepare a directory with a couple of lists
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer func() { assert.NoError(t, os.RemoveAll(dir)) }()

	goodData := `
		# Some comment
		121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@us-central-1.tardigrade.io:7777
		12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777
	`
	goodPath := filepath.Join(dir, "good.txt")
	require.NoError(t, ioutil.WriteFile(goodPath, []byte(goodData), 0644))

	badData := `BAD`
	badPath := filepath.Join(dir, "bad.txt")
	require.NoError(t, ioutil.WriteFile(badPath, []byte(badData), 0644))

	missingPath := filepath.Join(dir, "missing.txt")

	for _, tt := range []struct {
		name    string
		fileURL string
		err     string
		entries []Entry
	}{
		{
			name:    "bad list",
			fileURL: "file://" + badPath,
			err:     "trust: invalid satellite URL: must contain an ID",
		},
		{
			name:    "missing list",
			fileURL: "file://" + missingPath,
			err:     fmt.Sprintf("trust: open %s: no such file or directory", missingPath),
		},
		{
			name:    "good list",
			fileURL: "file://" + goodPath,
			entries: []Entry{
				{
					SatelliteURL:  url1,
					Authoritative: true,
				},
				{
					SatelliteURL:  url2,
					Authoritative: true,
				},
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewFileSource(tt.fileURL)
			require.NoError(t, err)
			entries, err := source.FetchEntries(context.Background())
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.entries, entries)
		})
	}
}

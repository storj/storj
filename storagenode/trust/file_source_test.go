// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/storagenode/trust"
)

func TestFileSourceString(t *testing.T) {
	source := trust.NewFileSource("/some/path")
	require.Equal(t, "/some/path", source.String())
}

func TestFileSourceIsStatic(t *testing.T) {
	source := trust.NewFileSource("/some/path")
	require.True(t, source.Static(), "file source is unexpectedly not static")
}

func TestFileSourceFetchEntries(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	url1 := makeSatelliteURL("domain1.test")
	url2 := makeSatelliteURL("domain2.test")

	// Prepare a directory with a couple of lists
	goodData := fmt.Sprintf(`
		# Some comment
		%s
		%s
	`, url1.String(), url2.String())
	goodPath := ctx.File("good.txt")
	require.NoError(t, ioutil.WriteFile(goodPath, []byte(goodData), 0644))

	badData := `BAD`
	badPath := ctx.File("bad.txt")
	require.NoError(t, ioutil.WriteFile(badPath, []byte(badData), 0644))

	missingPath := ctx.File("missing.txt")

	for _, tt := range []struct {
		name    string
		path    string
		err     string
		entries []trust.Entry
	}{
		{
			name: "bad list",
			path: badPath,
			err:  "file source: invalid satellite URL: must contain an ID",
		},
		{
			name: "missing list",
			path: missingPath,
			err:  fmt.Sprintf("file source: open %s: no such file or directory", missingPath),
		},
		{
			name: "good list",
			path: goodPath,
			entries: []trust.Entry{
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
			source := trust.NewFileSource(tt.path)
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

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSource(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		typ    interface{}
		err    string
	}{
		{
			name:   "HTTP source (using http)",
			config: "http://tardigrade.io",
			typ:    new(HTTPSource),
		},
		{
			name:   "HTTP source (using https)",
			config: "https://tardigrade.io",
			typ:    new(HTTPSource),
		},
		{
			name:   "file source",
			config: "file:///some/path",
			typ:    new(FileSource),
		},
		{
			name:   "not HTTP or FILE",
			config: "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@us-central-1.tardigrade.io:7777",
			typ:    new(FixedSource),
		},
		{
			name:   "not HTTP or FILE with bad URL",
			config: "OHNO!",
			err:    "trust: invalid satellite URL: must contain an ID",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewSource(tt.config)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.IsType(t, tt.typ, source)
		})
	}
}

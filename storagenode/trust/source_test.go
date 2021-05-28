// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/storagenode/trust"
)

func TestNewSource(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		typ    interface{}
		err    string
	}{
		{
			name:   "unrecognized schema",
			config: "ftp://domain.test",
			err:    `unsupported schema "ftp"`,
		},
		{
			name:   "HTTP source (using http)",
			config: "http://domain.test",
			typ:    new(trust.HTTPSource),
		},
		{
			name:   "HTTP source (using https)",
			config: "https://domain.test",
			typ:    new(trust.HTTPSource),
		},
		{
			name:   "relative file path",
			config: "path.txt",
			typ:    new(trust.FileSource),
		},
		{
			name:   "posix absolute file path",
			config: "/some/path.txt",
			typ:    new(trust.FileSource),
		},
		{
			name:   "windows absolute file path",
			config: "C:\\some\\path.txt",
			typ:    new(trust.FileSource),
		},
		{
			name:   "windows file path",
			config: "C:\\some\\path.txt",
			typ:    new(trust.FileSource),
		},
		{
			name:   "explicit satellite URL",
			config: "storj://121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@domain.test:7777",
			typ:    new(trust.StaticURLSource),
		},
		{
			name:   "explicit bad satellite URL",
			config: "storj://domain.test:7777",
			err:    "static source: invalid satellite URL: must contain an ID",
		},
		{
			name:   "satellite URL",
			config: "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@domain.test:7777",
			typ:    new(trust.StaticURLSource),
		},
		{
			name:   "partial satellite URL",
			config: "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@",
			err:    "static source: invalid satellite URL: must specify the host:port",
		},
		{
			name:   "partial satellite URL",
			config: "domain.test:7777",
			err:    "static source: invalid satellite URL: must contain an ID",
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			source, err := trust.NewSource(tt.config)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.IsType(t, tt.typ, source)
		})
	}
}

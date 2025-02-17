// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConnStr(t *testing.T) {
	t.Setenv("SPANNER_EMULATOR_HOST", "")

	tests := []struct {
		input   string
		want    ConnParams
		wantErr bool
	}{
		{
			input:   "",
			wantErr: true,
		},
		{
			input:   "postgres://user:secret@localhost",
			wantErr: true,
		},
		{
			input: "spanner://spanner:9010/projects/PROJECT/instances/INSTANCE/databases/DATABASE?emulator",
			want: ConnParams{
				Host:     "spanner:9010",
				Project:  "PROJECT",
				Instance: "INSTANCE",
				Database: "DATABASE",
				Emulator: true,
			},
		},
		{
			input:   "spanner://spanner:9010/projects/PROJECT/instances/INSTANCE/databases/DATABASE?invalid&emulator",
			wantErr: true,
		},
		{
			input: "spanner://spanner:9010/projects/PROJECT/instances/INSTANCE/databases/DATABASE",
			want: ConnParams{
				Host:     "spanner:9010",
				Project:  "PROJECT",
				Instance: "INSTANCE",
				Database: "DATABASE",
				Emulator: false,
			},
		},
		{
			input:   "spanner://spanner:9010/projects/PROJECT/instances/INSTANCE/databases/?emulator",
			wantErr: true,
		},
		{
			input: "spanner://spanner:9010/projects/PROJECT/instances/INSTANCE?emulator",
			want: ConnParams{
				Host:     "spanner:9010",
				Project:  "PROJECT",
				Instance: "INSTANCE",
				Emulator: true,
			},
		},
		{
			input:   "spanner://spanner:9010/projects/PROJECT/instances/",
			wantErr: true,
		},
		{
			input: "spanner://spanner:9010/projects/PROJECT?emulator",
			want: ConnParams{
				Host:     "spanner:9010",
				Project:  "PROJECT",
				Emulator: true,
			},
		},
		{
			input:   "spanner://spanner:9010/projects?emulator",
			wantErr: true,
		},
		{
			input: "spanner://spanner:9010?emulator",
			want: ConnParams{
				Host:     "spanner:9010",
				Emulator: true,
			},
		},
		{
			input: "spanner://localhost:9010",
			want: ConnParams{
				Host:     "localhost:9010",
				Emulator: true,
			},
		},
		{
			input: "spanner://127.0.0.1:9010",
			want: ConnParams{
				Host:     "127.0.0.1:9010",
				Emulator: true,
			},
		},
		{
			input: "spanner://projects/PROJECT/instances/INSTANCE/databases/DATABASE",
			want: ConnParams{
				Project:  "PROJECT",
				Instance: "INSTANCE",
				Database: "DATABASE",
				Emulator: false,
			},
		},
		{
			input:   "spanner://projects/PROJECT/instances/INSTANCE/databases/?emulator",
			wantErr: true,
		},
		{
			input: "spanner://projects/PROJECT/instances/INSTANCE?emulator",
			want: ConnParams{
				Project:  "PROJECT",
				Instance: "INSTANCE",
				Emulator: true,
			},
		},
		{
			input:   "spanner://projects/PROJECT/instances/",
			wantErr: true,
		},
		{
			input: "spanner://projects/PROJECT?emulator",
			want: ConnParams{
				Project:  "PROJECT",
				Emulator: true,
			},
		},
		{
			input:   "spanner://projects?emulator",
			wantErr: true,
		},
		{
			input: "spanner://spanner:9010?create-instance.display-name=Hello&create-instance.config=us1&create-instance.node-count=3&create-instance.processing-units=100",
			want: ConnParams{
				Host: "spanner:9010",
				CreateInstance: ConnParamsCreateInstance{
					DisplayName:     "Hello",
					Config:          "us1",
					NodeCount:       3,
					ProcessingUnits: 100,
				},
			},
		},
	}

	for _, test := range tests {
		got, err := ParseConnStr(test.input)
		if test.wantErr {
			if err == nil {
				t.Errorf("%q: wanted error, but got %v", test.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: parsing failed, %v", test.input, err)
			continue
		}

		assert.Equal(t, test.want, got)
	}
}

func TestConnParams_String(t *testing.T) {
	params := ConnParams{
		Host: "spanner:9010",
		CreateInstance: ConnParamsCreateInstance{
			DisplayName:     "Hello",
			Config:          "us1",
			NodeCount:       3,
			ProcessingUnits: 100,
		},
	}

	require.Equal(t, "spanner://spanner:9010?create-instance.config=us1&create-instance.display-name=Hello&create-instance.node-count=3&create-instance.processing-units=100", params.ConnStr())
}

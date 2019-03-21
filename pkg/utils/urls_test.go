// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import "testing"

func TestValidateURL(t *testing.T) {
	type args struct {
		dst string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test happy path",
			args: args{
				dst: "sj://test/bucket",
			},
			want: true,
		},
		{
			name: "test triple slash from url parse",
			args: args{
				dst: "sj:///test",
			},
			want: false,
		},
		{
			name: "test single slash",
			args: args{
				dst: "sj:/test/bucket",
			},
			want: false,
		},
		{
			name: "test s3 scheme",
			args: args{
				dst: "s3://test/s3",
			},
			want: true,
		},
		{
			name: "test bucket only",
			args: args{
				dst: "sj://bucket",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateURL(tt.args.dst); got != tt.want {
				t.Errorf("ValidateURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

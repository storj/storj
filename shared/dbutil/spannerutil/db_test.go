// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"fmt"
	"reflect"
	"testing"
)

func TestBuildURL(t *testing.T) {
	testProject := "test"
	testInstance := "instance"
	testDatabase := "database"

	type args struct {
		project  string
		instance string
		database *string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "full url with database succeeds",
			args: args{
				project:  testProject,
				instance: testInstance,
				database: &testDatabase,
			},
			want: fmt.Sprintf("spanner://projects/%s/instances/%s/databases/%s", testProject, testInstance, testDatabase),
		},
		{
			name: "full url without database succeeds",
			args: args{
				project:  testProject,
				instance: testInstance,
				database: nil,
			},
			want: fmt.Sprintf("spanner://projects/%s/instances/%s", testProject, testInstance),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildURL(tt.args.project, tt.args.instance, tt.args.database); got != tt.want {
				t.Errorf("BuildURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConnStr(t *testing.T) {
	testProject := "test"
	testInstance := "instance"
	testDatabase := "database"
	testURL := fmt.Sprintf("spanner://projects/%s/instances/%s/databases/%s", testProject, testInstance, testDatabase)
	partialURL := fmt.Sprintf("spanner://projects/%s/instances/%s", testProject, testInstance)
	badURL := fmt.Sprintf("spaner://project/%s/instance/%s/database/%s", testProject, testInstance, testDatabase)
	postgresURL := "postgres://user:secret@localhost"

	type args struct {
		full string
	}
	tests := []struct {
		name         string
		args         args
		wantProject  string
		wantInstance string
		wantDatabase *string
		wantErr      bool
	}{
		{
			name:         "full url connection string parsed successfully",
			args:         args{full: testURL},
			wantProject:  testProject,
			wantInstance: testInstance,
			wantDatabase: &testDatabase,
			wantErr:      false,
		},
		{
			name:         "partial url connection string parsed successfully",
			args:         args{full: partialURL},
			wantProject:  testProject,
			wantInstance: testInstance,
			wantDatabase: nil,
			wantErr:      false,
		},
		{
			name:         "bad url connection string errors",
			args:         args{full: badURL},
			wantProject:  "",
			wantInstance: "",
			wantDatabase: nil,
			wantErr:      true,
		},
		{
			name:         "postgres url connection string errors",
			args:         args{full: postgresURL},
			wantProject:  "",
			wantInstance: "",
			wantDatabase: nil,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProject, gotInstance, gotDatabase, err := ParseConnStr(tt.args.full)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConnStr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotProject != tt.wantProject {
				t.Errorf("ParseConnStr() gotProject = %v, want %v", gotProject, tt.wantProject)
			}
			if gotInstance != tt.wantInstance {
				t.Errorf("ParseConnStr() gotInstance = %v, want %v", gotInstance, tt.wantInstance)
			}
			if !reflect.DeepEqual(gotDatabase, tt.wantDatabase) {
				t.Errorf("ParseConnStr() gotDatabase = %v, want %v", gotDatabase, tt.wantDatabase)
			}
		})
	}
}

func TestDSNFromURL(t *testing.T) {
	dsn := "projects/test-project/instances/test-instance/databases/test-database"
	fullURL := "spanner://" + dsn

	type args struct {
		url string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "full url returns dsn",
			args: args{url: fullURL},
			want: dsn,
		},
		{
			name: "dsn is unchanged",
			args: args{url: dsn},
			want: dsn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DSNFromURL(tt.args.url); got != tt.want {
				t.Errorf("DSNFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEscapeIdentifier(t *testing.T) {
	hyphen := "test-this-out"
	underscores := "test_this_out"

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "string with hyphens is escaped successfully",
			input: hyphen,
			want:  "`" + hyphen + "`",
		},
		{
			name:  "string with underscores is escaped successfully",
			input: underscores,
			want:  "`" + underscores + "`",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeIdentifier(tt.input); got != tt.want {
				t.Errorf("EscapeIdentifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

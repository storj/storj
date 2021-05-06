// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/cmd/uplinkng/ulloc"
)

// Result captures all the output of running a command for inspection.
type Result struct {
	Stdout     string
	Stderr     string
	Ok         bool
	Err        error
	Operations []Operation
}

// RequireSuccess fails if the Result did not observe a successful execution.
func (r Result) RequireSuccess(t *testing.T) {
	if !r.Ok {
		errs := parseErrors(r.Stdout)
		require.True(t, r.Ok, "test did not run successfully. errors:\n%s",
			strings.Join(errs, "\n"))
	}
	require.NoError(t, r.Err)
}

// RequireFailure fails if the Result did not observe a failed execution.
func (r Result) RequireFailure(t *testing.T) {
	require.False(t, r.Ok && r.Err == nil, "command ran with no error")
}

// RequireStdout requires that the execution wrote to stdout the provided string.
// Blank lines are ignored and all lines are space trimmed for the comparison.
func (r Result) RequireStdout(t *testing.T, stdout string) {
	require.Equal(t, trimNewlineSpaces(stdout), trimNewlineSpaces(r.Stdout))
}

// RequireStderr requires that the execution wrote to stderr the provided string.
// Blank lines are ignored and all lines are space trimmed for the comparison.
func (r Result) RequireStderr(t *testing.T, stderr string) {
	require.Equal(t, trimNewlineSpaces(stderr), trimNewlineSpaces(r.Stderr))
}

func parseErrors(s string) []string {
	lines := strings.Split(s, "\n")
	start := 0
	for i, line := range lines {
		if line == "Errors:" {
			start = i + 1
		} else if len(line) > 0 && line[0] != ' ' {
			return lines[start:i]
		}
	}
	return nil
}

func trimNewlineSpaces(s string) string {
	lines := strings.Split(s, "\n")

	j := 0
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); len(trimmed) > 0 {
			lines[j] = trimmed
			j++
		}
	}
	return strings.Join(lines[:j], "\n")
}

// Operation represents some kind of filesystem operation that happened
// on some location, and if the operation failed.
type Operation struct {
	Kind  string
	Loc   string
	Error bool
}

func newOp(kind string, loc ulloc.Location, err error) Operation {
	return Operation{
		Kind:  kind,
		Loc:   loc.String(),
		Error: err != nil,
	}
}

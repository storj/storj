// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"testing"
)

func TestSnakeCase(t *testing.T) {
	for _, test := range []struct {
		input, expected string
	}{
		{"CoolBeans", "cool_beans"},
		{"coolBeans", "cool_beans"},
		{"JSONBeans", "json_beans"},
		{"JSONBeAns", "json_be_ans"},
		{"JSONBeANS", "json_be_ans"},
		{"coolbeans", "coolbeans"},
		{"COOLBEANS", "coolbeans"},
		{"CoolJSON", "cool_json"},
		{"CoolJSONBeans", "cool_json_beans"},
	} {
		actual := snakeCase(test.input)
		if actual != test.expected {
			t.Logf("expected %#v but got %#v", test.expected, actual)
			t.Fail()
		}
	}
}

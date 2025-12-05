// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package utils_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
)

func TestEmailValidation(t *testing.T) {
	invalidEmailAddresses := []string{
		"test@t@t.test",
		"test",
		"test@!t.test",
		"test@#test.test",
		"test@$t.test",
		"t%t.test",
		"test@^test.test",
		"test@&test.test",
		"test@*test.test",
		"test@(test.test",
		"test@)test.test",
		"test@=test.test",
		"test@[test.test",
		"test@]test.test",
		"test@{test.test",
		"test@}test.test",
		"test@/test.test",
		"test@\\test.test",
		"test@|test.test",
		"test@:test.test",
		"test@;test.test",
		"test@,test.test",
		"test@\"test.test",
		"test@'test.test",
		"test@<test.test",
		"test@>test.test",
		"test@_test.test",
		"test@?test.test",
	}

	for _, e := range invalidEmailAddresses {
		result := utils.ValidateEmail(e)
		require.False(t, result)
	}
}

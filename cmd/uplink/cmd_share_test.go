// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/cmd/uplink/ultest"
)

func TestShare(t *testing.T) {
	t.Run("share requires prefix", func(t *testing.T) {
		ultest.Setup(commands).Fail(t, "share")
	})

	t.Run("share default access", func(t *testing.T) {
		state := ultest.Setup(commands)

		state.Succeed(t, "share", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download  : Allowed
			Upload    : Disallowed
			Lists     : Allowed
			Deletes   : Disallowed
			NotBefore : No restriction
			NotAfter  : No restriction
			Paths     : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access    : *
		`)
	})

	t.Run("share access with --readonly", func(t *testing.T) {
		state := ultest.Setup(commands)

		state.Succeed(t, "share", "--readonly", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download  : Allowed
			Upload    : Disallowed
			Lists     : Allowed
			Deletes   : Disallowed
			NotBefore : No restriction
			NotAfter  : No restriction
			Paths     : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access    : *
		`)
	})

	t.Run("share access with --disallow-lists", func(t *testing.T) {
		state := ultest.Setup(commands)

		state.Succeed(t, "share", "--disallow-lists", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download  : Allowed
			Upload    : Disallowed
			Lists     : Disallowed
			Deletes   : Disallowed
			NotBefore : No restriction
			NotAfter  : No restriction
			Paths     : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access    : *
		`)
	})

	t.Run("share access with --disallow-reads", func(t *testing.T) {
		state := ultest.Setup(commands)

		state.Succeed(t, "share", "--disallow-reads", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download  : Disallowed
			Upload    : Disallowed
			Lists     : Allowed
			Deletes   : Disallowed
			NotBefore : No restriction
			NotAfter  : No restriction
			Paths     : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access    : *
		`)
	})

	t.Run("share access with --writeonly", func(t *testing.T) {
		state := ultest.Setup(commands)

		result := state.Fail(t, "share", "--writeonly", "sj://some/prefix")

		require.Equal(t, "permission is empty", result.Err.Error())
	})

	t.Run("share access with --public", func(t *testing.T) {
		// Can't run this scenario because AuthService is not running in testplanet.
		// If necessary we can mock AuthService like in https://github.com/storj/uplink/blob/main/testsuite/edge_test.go
		t.Skip("No AuthService available in testplanet")
		state := ultest.Setup(commands)

		state.Succeed(t, "share", "--public", "--not-after=none", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download  : Allowed
			Upload    : Disallowed
			Lists     : Allowed
			Deletes   : Disallowed
			NotBefore : No restriction
			NotAfter  : No restriction
			Paths     : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access    : *
		`)
	})

	t.Run("share access with --not-after time restriction parameter", func(t *testing.T) {
		state := ultest.Setup(commands)

		state.Succeed(t, "share", "--not-after", "2022-01-01T15:01:01-01:00", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download  : Allowed
			Upload    : Disallowed
			Lists     : Allowed
			Deletes   : Disallowed
			NotBefore : No restriction
			NotAfter  : 2022-01-01 16:01:01
			Paths     : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access    : *
		`)
	})
}

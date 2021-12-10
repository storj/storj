// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/cmd/uplinkng/ultest"
	"storj.io/uplink"
)

func TestShare(t *testing.T) {
	access := "12edqrJX1V243n5fWtUrwpMQXL8gKdY2wbyqRPSG3rsA1tzmZiQjtCyF896egifN2C2qdY6g5S1t6e8iDhMUon9Pb7HdecBFheAcvmN8652mqu8hRx5zcTUaRTWfFCKS2S6DHmTeqPUHJLEp6cJGXNHcdqegcKfeahVZGP4rTagHvFGEraXjYRJ3knAcWDGW6BxACqogEWez6r274JiUBfs4yRSbRNRqUEURd28CwDXMSHLRKKA7TEDKEdQ"

	t.Run("share default access", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		result := state.Succeed(t, "share", "--access", access)

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Allowed
		Upload    : Disallowed
		Lists     : Allowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : No restriction
		Paths     : WARNING! The entire project is shared!
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})

	t.Run("share access with --readonly", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		result := state.Succeed(t, "share", "--access", access, "--readonly")

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Allowed
		Upload    : Disallowed
		Lists     : Allowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : No restriction
		Paths     : WARNING! The entire project is shared!
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})

	t.Run("share access with --disallow-lists", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		result := state.Succeed(t, "share", "--access", access, "--disallow-lists")

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Allowed
		Upload    : Disallowed
		Lists     : Disallowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : No restriction
		Paths     : WARNING! The entire project is shared!
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})

	t.Run("share access with --disallow-reads", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		result := state.Succeed(t, "share", "--access", access, "--disallow-reads")

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Disallowed
		Upload    : Disallowed
		Lists     : Allowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : No restriction
		Paths     : WARNING! The entire project is shared!
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})

	t.Run("share access with --writeonly", func(t *testing.T) {
		state := ultest.Setup(commands)

		result := state.Fail(t, "share", "--access", access, "--writeonly")
		assert.Equal(t, "permission is empty", result.Err.Error())
	})

	t.Run("share access with --public", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		result := state.Succeed(t, "share", "--public", "--not-after=none")

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Allowed
		Upload    : Disallowed
		Lists     : Allowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : No restriction
		Paths     : WARNING! The entire project is shared!
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})

	t.Run("share access with --not-after time restriction parameter", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		notAfterDate := "2022-01-01T15:01:01-01:00"
		result := state.Succeed(t, "share", "--access", access, "--not-after", notAfterDate)

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Allowed
		Upload    : Disallowed
		Lists     : Allowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : 2022-01-01 16:01:01
		Paths     : WARNING! The entire project is shared!
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})

	t.Run("share access with prefix", func(t *testing.T) {
		state := ultest.Setup(commands)

		acc, err := uplink.ParseAccess(access)
		assert.NoError(t, err)

		result := state.Succeed(t, "share", "--access", access, "sj://bucket/object-to-share")

		// TODO we need to find nicer way to compare results
		accessIndex := strings.Index(result.Stdout, "Access    :")
		result.Stdout = result.Stdout[:accessIndex] //nolint: gocritic

		result.RequireStdout(t, `
		Sharing access to satellite `+acc.SatelliteAddress()+`
		=========== ACCESS RESTRICTIONS ==========================================================
		Download  : Allowed
		Upload    : Disallowed
		Lists     : Allowed
		Deletes   : Disallowed
		NotBefore : No restriction
		NotAfter  : No restriction
		Paths     : sj://bucket/object-to-share
		=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
		`)
	})
}

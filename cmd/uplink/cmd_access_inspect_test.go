// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestAccessInspect(t *testing.T) {
	parsedAccessA :=
		`
			{
				"satellite_addr": "12V4jtJhKFNoUtHNG9VaTPEn5MyeHvbNdT2UtfqN8qWN6ATd7FX@storjsim:10000",
				"encryption_access": {
					"default_key": "N87DsbLnjfzhTVxq1NY5kb68htyFoQ3vpf3+PXOryrk=",
					"default_path_cipher": "ENC_AESGCM"
				},
				"api_key": "13YqdsDpS5T26sbKUPWkqeczTN1TEm28hBuCD5mCGEDsC9JNte3VUbj2iJRsWogorpfbm3cCx8gFfAWPtf3tJncy9YRjU3734VKPYoh",
				"macaroon": {
					"head": "QaR7H0JWYExN1uELObuXuPkkgICizvci5DCGPQvKQ2I=",
					"caveats": [],
					"tail": "STxHftTfJh9El-dvL6fZC-J3ikE5HAP8Th-e2KYTcqA="
				}
			}
		`
	parsedAccessB :=
		`{
		"satellite_addr": "1d1wmTEDe994p1McyYwVvfR5PeK8mqq4hfvJ8LyZWTDNuhZtnw@127.0.0.1:10000",
		"encryption_access": {
			"default_key": "4b2E3n2lczSd529FngLUMhG8gr7W0KpllZiFDEcl/08=",
			"default_path_cipher": "ENC_AESGCM"
		},
		"api_key": "1dfJRDAYwLTD3Repgzg6DS1gABme4BQXTp8mmQ56penApYPJ8bMLmF6125scmj7PVKezhSraMGnU6WiwGwPpe6u9Vq5tqZMJD13bnUB4hykKaTaNptRY",
		"macaroon": {
		"head": "nXhdVngN8Q2oNfro0vqCBvJ8WXHJ76aTGElbrImcwJQ=",
		"caveats": [
			{
				"nonce": "v1MmWA=="
			}
		],
		"tail": "WiJnEHpTzPzBjfR2dxUdeXOdWe-zQROjQywZ1gk6Cbg="
		}
	}`
	accessValue := "12edqrJX1V243n5fWtUrwpMQXL8gKdY2wbyqRPSG3rsA1tzmZiQjtCyF896egifN2C2qdY6g5S1t6e8iDhMUon9Pb7HdecBFheAcvmN8652mqu8hRx5zcTUaRTWfFCKS2S6DHmTeqPUHJLEp6cJGXNHcdqegcKfeahVZGP4rTagHvFGEraXjYRJ3knAcWDGW6BxACqogEWez6r274JiUBfs4yRSbRNRqUEURd28CwDXMSHLRKKA7TEDKEdQ"

	state := ultest.Setup(uplinkcli.Commands)

	t.Run("get first valid access by name", func(t *testing.T) {
		state.Succeed(t, "access", "inspect", "TestAccessA").RequireStdout(t, parsedAccessA)
	})

	t.Run("get second valid access by name", func(t *testing.T) {
		state.Succeed(t, "access", "inspect", "TestAccessB").RequireStdout(t, parsedAccessB)
	})

	t.Run("get first valid access by value", func(t *testing.T) {
		state.Succeed(t, "access", "inspect", accessValue).RequireStdout(t, parsedAccessA)
	})

	t.Run("get default access, calling without parameters", func(t *testing.T) {
		state.Succeed(t, "access", "inspect").RequireStdout(t, parsedAccessA)
	})

	t.Run("try to get unexisting access", func(t *testing.T) {
		state.Fail(t, "access", "inspect", "unexisting")
	})
}

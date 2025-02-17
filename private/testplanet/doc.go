// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

// Package testplanet implements full network wiring for testing.
//
// testplanet provides access to most of the internals of satellites,
// storagenodes and uplinks.
//
// # Database
//
// It does require setting two variables for the databases:
//
//	STORJ_TEST_POSTGRES=postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable
//	STORJ_TEST_COCKROACH=cockroach://root@localhost:26257/master?sslmode=disable
//	STORJ_TEST_SPANNER=spanner://localhost:9010?emulator
//
// When you wish to entirely omit either of them from the test output, it's possible to use:
//
//	STORJ_TEST_POSTGRES=omit
//	STORJ_TEST_COCKROACH=omit
//
// # Host
//
// It's possible to change the listing host with:
//
//	STORJ_TEST_HOST=127.0.0.2;127.0.0.3
//
// # Debugging
//
// For debugging, it's possible to set STORJ_TEST_MONKIT to get a trace per test.
//
//	STORJ_TEST_MONKIT=svg
//	STORJ_TEST_MONKIT=json
//
// By default, it saves the output the same folder as the test. However, if you wish
// to specify a separate folder, you can specify an absolute directory:
//
//	STORJ_TEST_MONKIT=svg,dir=/home/user/debug/trace
//
// Note, due to how go tests work, it's not possible to specify a relative directory.
package testplanet

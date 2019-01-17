// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcmd"
	"storj.io/storj/internal/testcontext"
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	if *testcmd.Integration {
		m.Run()
	}
}

func TestCmdCreateAuth(t *testing.T) {
	assert := assert.New(t)
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	commands, err := testcmd.Build(ctx, testcmd.CmdCertificates)
	if !assert.NoError(err) {
		t.Fatal(err)
	}

	// `certificates auth create 1 user@example.com`
	cases := map[string]int{
		"one@example.com": 1,
		"two@example.com": 2,
		"ten@example.com": 10,
	}

	for userID, count := range cases {
		err := commands[testcmd.CmdCertificates].Run("auth", "create", strconv.Itoa(count), userID)
		if !assert.NoError(err) {
			t.Fatal(err)
		}
	}

}

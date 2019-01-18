// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
)

func TestCreateAuth(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	certificatesexe := ctx.Compile("storj.io/storj/cmd/certificates")

	// `certificates auth create 1 user@example.com`
	cases := map[string]int{
		"one@example.com": 1,
		"two@example.com": 2,
		"ten@example.com": 10,
	}

	for userID, count := range cases {
		data, err := exec.Command(certificatesexe,
			"--config-dir", ctx.Dir(userID),
			"auth", "create", strconv.Itoa(count), userID).CombinedOutput()
		t.Log(string(data))
		assert.NoError(t, err)
	}

}

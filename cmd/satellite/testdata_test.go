// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTestApiKey(t *testing.T) {
	key, err := GetTestApiKey("1nxk8B3L3zJi1nGTDemMWSTiyTehu87XaMGy1f742igrTEPciS@localhost:10001")
	require.NoError(t, err)
	require.Equal(t, "14CuLjd1ypmCQGkwFCAWxPGVRugM52shEia6MS2ASSTJ6P2qKsW2fWEujLH8ZaycTGHdxSpXoAWxXcwCsueJdCRcj2E4MHysMiwWsmm9rLnPYjE9YwUkNgDnR4nJHNtBbBF94smgk52RQw9WHHxas6owWJJceKmTYkip3odDHW52dNVqJVFRZZqwx4WEorey9KBe95RmSNbCL3mp6LVcMTYTT8vHSzPPAaT54wGuxiSH2rFkzkHBnL7fC9v6E6516pdXJLzCkzbTXwiifWXpiZggTMuaGPyhuBpSZMF2o4Sa", key)
}

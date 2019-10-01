// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMapConfigs(t *testing.T) {
	log := zap.L()

	cases := []struct {
		testID          string
		newExternalAddr string
		oldExternalAddr string
		expectedAddr    string
		newWallet       string
		oldWallet       string
		expectedWallet  string
		newEmail        string
		oldEmail        string
		expectedEmail   string
	}{
		{testID: "new and old present, use new",
			newExternalAddr: "newAddr",
			oldExternalAddr: "oldAddr",
			expectedAddr:    "newAddr",
			newWallet:       "newWallet",
			oldWallet:       "oldWallet",
			expectedWallet:  "newWallet",
			newEmail:        "newEmail",
			oldEmail:        "oldEmail",
			expectedEmail:   "newEmail",
		},
		{testID: "old present, new not, use old",
			newExternalAddr: "",
			oldExternalAddr: "oldAddr",
			expectedAddr:    "oldAddr",
			newWallet:       "",
			oldWallet:       "oldWallet",
			expectedWallet:  "oldWallet",
			newEmail:        "",
			oldEmail:        "oldEmail",
			expectedEmail:   "oldEmail",
		},
		{testID: "new present, old not, use new",
			newExternalAddr: "newAddr",
			oldExternalAddr: "",
			expectedAddr:    "newAddr",
			newWallet:       "newWallet",
			oldWallet:       "",
			expectedWallet:  "newWallet",
			newEmail:        "newEmail",
			oldEmail:        "",
			expectedEmail:   "newEmail",
		},
		{testID: "neither present",
			newExternalAddr: "",
			oldExternalAddr: "",
			expectedAddr:    "",
			newWallet:       "",
			oldWallet:       "",
			expectedWallet:  "",
			newEmail:        "",
			oldEmail:        "",
			expectedEmail:   "",
		},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			runCfg.Contact.ExternalAddress = testCase.newExternalAddr
			runCfg.Kademlia.ExternalAddress = testCase.oldExternalAddr
			runCfg.Operator.Wallet = testCase.newWallet
			runCfg.Kademlia.Operator.Wallet = testCase.oldWallet
			runCfg.Operator.Email = testCase.newEmail
			runCfg.Kademlia.Operator.Email = testCase.oldEmail
			mapDeprecatedConfigs(log)
			require.Equal(t, testCase.expectedAddr, runCfg.Contact.ExternalAddress)
			require.Equal(t, testCase.expectedWallet, runCfg.Operator.Wallet)
			require.Equal(t, testCase.expectedEmail, runCfg.Operator.Email)
		})
	}
}

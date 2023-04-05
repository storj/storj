// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMapDeprecatedConfigs(t *testing.T) {
	log := zap.L()

	cases := []struct {
		testID                 string
		newExternalAddr        string
		deprecatedExternalAddr string
		expectedAddr           string
		newWallet              string
		deprecatedWallet       string
		expectedWallet         string
		newEmail               string
		deprecatedEmail        string
		expectedEmail          string
	}{
		{testID: "deprecated present, override",
			newExternalAddr:        "newAddr",
			deprecatedExternalAddr: "oldAddr",
			expectedAddr:           "oldAddr",
			newWallet:              "newWallet",
			deprecatedWallet:       "oldWallet",
			expectedWallet:         "oldWallet",
			newEmail:               "newEmail",
			deprecatedEmail:        "oldEmail",
			expectedEmail:          "oldEmail",
		},
		{testID: "deprecated absent, do not override",
			newExternalAddr:        "newAddr",
			deprecatedExternalAddr: "",
			expectedAddr:           "newAddr",
			newWallet:              "newWallet",
			deprecatedWallet:       "",
			expectedWallet:         "newWallet",
			newEmail:               "newEmail",
			deprecatedEmail:        "",
			expectedEmail:          "newEmail",
		},
	}
	runCfg := runCfg{}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			runCfg.Contact.ExternalAddress = testCase.newExternalAddr
			runCfg.Deprecated.Kademlia.ExternalAddress = testCase.deprecatedExternalAddr
			runCfg.Operator.Wallet = testCase.newWallet
			runCfg.Deprecated.Kademlia.Operator.Wallet = testCase.deprecatedWallet
			runCfg.Operator.Email = testCase.newEmail
			runCfg.Deprecated.Kademlia.Operator.Email = testCase.deprecatedEmail
			mapDeprecatedConfigs(log, &runCfg.StorageNodeFlags)
			require.Equal(t, testCase.expectedAddr, runCfg.Contact.ExternalAddress)
			require.Equal(t, testCase.expectedWallet, runCfg.Operator.Wallet)
			require.Equal(t, testCase.expectedEmail, runCfg.Operator.Email)
		})
	}
}

func TestParseOverride(t *testing.T) {
	cases := []struct {
		testID            string
		newConfigValue    interface{}
		oldConfigValue    string
		expectedNewConfig interface{}
	}{
		{testID: "test new config is untouched if deprecated is undefined",
			newConfigValue:    "test-string",
			oldConfigValue:    "",
			expectedNewConfig: "test-string",
		},
		{testID: "test migrate int",
			newConfigValue:    int(1),
			oldConfigValue:    "100",
			expectedNewConfig: int(100),
		},
		{testID: "test migrate int64",
			newConfigValue:    int64(1),
			oldConfigValue:    "100",
			expectedNewConfig: int64(100),
		},
		{testID: "test migrate uint",
			newConfigValue:    uint(1),
			oldConfigValue:    "2",
			expectedNewConfig: uint(2),
		},
		{testID: "test migrate uint64",
			newConfigValue:    uint64(1),
			oldConfigValue:    "100",
			expectedNewConfig: uint64(100),
		},
		{testID: "test migrate time.Duration",
			newConfigValue:    1 * time.Hour,
			oldConfigValue:    "100h",
			expectedNewConfig: 100 * time.Hour,
		},
		{testID: "test migrate float64",
			newConfigValue:    0.5,
			oldConfigValue:    "1.5",
			expectedNewConfig: 1.5,
		},
		{testID: "test migrate string",
			newConfigValue:    "example@example.com",
			oldConfigValue:    "migrate@migrate.com",
			expectedNewConfig: "migrate@migrate.com",
		},
		{testID: "test migrate bool",
			newConfigValue:    false,
			oldConfigValue:    "true",
			expectedNewConfig: true,
		},
	}
	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			if testCase.oldConfigValue != "" {
				typ := reflect.TypeOf(testCase.newConfigValue)
				testCase.newConfigValue = parseOverride(typ, testCase.oldConfigValue)
			}
			require.Equal(t, testCase.expectedNewConfig, testCase.newConfigValue)
		})
	}
}

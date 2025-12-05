// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package sso_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/console/consoleauth/sso"
)

func TestOidcInfoConfigValidation(t *testing.T) {
	url1, err := url.Parse("https://metadata1.com")
	require.NoError(t, err)
	url2, err := url.Parse("https://metadata2.com")
	require.NoError(t, err)
	tests := []struct {
		description    string
		configString   string
		expectedConfig sso.OidcProviderInfos
		expectError    bool
	}{
		{
			description:  "valid config string",
			configString: fmt.Sprintf("provider1:client-id,client-secret,%s;provider2:client-id,client-secret,%s", url1.String(), url2.String()),
			expectedConfig: sso.OidcProviderInfos{
				Values: map[string]sso.OidcProviderInfo{
					"provider1": {
						ClientID:     "client-id",
						ClientSecret: "client-secret",
						ProviderURL:  *url1,
					},
					"provider2": {
						ClientID:     "client-id",
						ClientSecret: "client-secret",
						ProviderURL:  *url2,
					},
				},
			},
			expectError: false,
		},
		{
			description:  "invalid config string",
			configString: fmt.Sprintf("provider1:%s;provider2:%s", url1.String(), url2.String()),
			expectError:  true,
		},
		{
			description:  "invalid url string",
			configString: "provider1:client-id,client-secret,:://invalid.com",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Log(tt.description)

		info := sso.OidcProviderInfos{}
		err := info.Set(tt.configString)
		if tt.expectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.EqualValues(t, tt.expectedConfig.Values, info.Values)
		}
	}
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/payments/coinpayments"
)

func TestGetCheckoutURL(t *testing.T) {
	expected := "example"

	url := coinpayments.GetCheckoutURL(expected, "id")

	key, err := coinpayments.GetTransacationKeyFromURL(url)
	require.NoError(t, err)

	assert.Equal(t, expected, key)
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/payments/coinpayments"
)

func TestGetCheckoutURL(t *testing.T) {
	expected := "example"

	link := coinpayments.GetCheckoutURL(expected, "id")

	u, err := url.Parse(link)
	require.NoError(t, err)

	assert.Equal(t, expected, u.Query().Get("key"))
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

type CreateRequest struct {
	Path   string
	Values url.Values
}

func TestCreateOffer(t *testing.T) {
	// We can't call this in testplanet
	t.Parallel()

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		requests := []CreateRequest{
			{
				Path: "/create/referral-offer",
				Values: url.Values{
					"Name":                      {"Referral Credit"},
					"Description":               {"desc"},
					"ExpiresAt":                 {"2019-06-27"},
					"InviteeCreditInCents":      {"50"},
					"InviteeCreditDurationDays": {"50"},
					"AwardCreditInCents":        {"50"},
					"AwardCreditDurationDays":   {"50"},
					"RedeemableCap":             {"150"},
				},
			}, {
				Path: "/create/free-credit-offer",
				Values: url.Values{
					"Name":                      {"Free Credit Credit"},
					"Description":               {"desc"},
					"ExpiresAt":                 {"2019-06-27"},
					"InviteeCreditInCents":      {"50"},
					"InviteeCreditDurationDays": {"50"},
					"RedeemableCap":             {"150"},
				},
			},
		}

		for _, offer := range requests {

			addr := planet.Satellites[0].Marketing.Listener.Addr()

			url := "http://" + addr.String() + offer.Path

			resp, err := http.PostForm(url, offer.Values)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})
}

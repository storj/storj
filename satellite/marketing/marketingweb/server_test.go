// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/marketing"
)

func buildForm(isReferralOffer bool) url.Values {
	form := url.Values{}
	form.Add("Name", "May Credit")
	form.Add("Description", "desc")
	form.Add("ExpiresAt", "2019-06-27")
	form.Add("InviteeCreditInCents", "50")
	form.Add("InviteeCreditDurationDays", "50")
	form.Add("RedeemableCap", "150")

	if isReferralOffer {
		form.Add("AwardCreditInCents", "50")
		form.Add("AwardCreditDurationDays", "50")
	}
	return form
}

func buildResources(address, endpoint string, isReferralOffer bool) (url.URL, url.Values, error) {
	URL, err := url.ParseRequestURI("http://" + address)
	if err != nil {
		fmt.Printf("err from buildResources : %v\n", err)
		return *URL, url.Values{}, err
	}
	URL.Path = endpoint
	form := buildForm(isReferralOffer)
	URL.RawQuery = form.Encode()
	return *URL, form, nil
}

func TestCreateOffer(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		validOffers := []marketing.NewOffer{
			{Type: marketing.Referral},
			{Type: marketing.FreeCredit},
		}

		s := planet.Satellites[0].Marketing.Endpoint

		for _, offer := range validOffers {

			var (
				form            url.Values
				isReferralOffer bool
			)

			endpoint := "/create"

			switch offer.Type {

			case marketing.Referral:
				endpoint += "/referral-offer"
				isReferralOffer = true
			case marketing.FreeCredit:
				isReferralOffer = false
				endpoint += "/free-credit"
			}

			URL, form, err := buildResources(s.Config.Address, endpoint, isReferralOffer)
			require.NoError(t, err, "failed to build request resources")

			urlStr := URL.String()

			req, err := http.NewRequest("POST", urlStr, strings.NewReader(URL.RawQuery))
			require.NoError(t, err, "failed to create new POST request")

			req.PostForm = form
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			rr := httptest.NewRecorder()

			go func(recorder *httptest.ResponseRecorder, request *http.Request) {

				s.CreateOffer(recorder, request)

				resp := recorder.Result()
				if resp.StatusCode != http.StatusSeeOther {
					t.Fatalf("Received StatusCode %d expected %d\n", resp.StatusCode, http.StatusSeeOther)
				}
			}(rr, req)
		}
	})
}

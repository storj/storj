// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

type CreateRequest struct {
	Path   string
	Values url.Values
}

func TestCreateAndStopOffers(t *testing.T) {
	t.Skip("this test will be removed/modified with rework of offer/rewards code")
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		requests := []CreateRequest{
			{
				Path: "/create/referral",
				Values: url.Values{
					"Name":                      {"Referral Credit"},
					"Description":               {"desc"},
					"ExpiresAt":                 {"2119-06-27"},
					"InviteeCredit":             {"50"},
					"InviteeCreditDurationDays": {"50"},
					"AwardCredit":               {"50"},
					"AwardCreditDurationDays":   {"50"},
					"RedeemableCap":             {"150"},
				},
			}, {
				Path: "/create/free-credit",
				Values: url.Values{
					"Name":                      {"Free Credit"},
					"Description":               {"desc"},
					"ExpiresAt":                 {"2119-06-27"},
					"InviteeCredit":             {"50"},
					"InviteeCreditDurationDays": {"50"},
					"RedeemableCap":             {"150"},
				},
			}, {
				Path: "/create/partner",
				Values: url.Values{
					"Name":                      {"FileZilla"},
					"Description":               {"desc"},
					"ExpiresAt":                 {"2119-06-27"},
					"InviteeCredit":             {"50"},
					"InviteeCreditDurationDays": {"50"},
					"RedeemableCap":             {"150"},
				},
			},
		}

		addr := planet.Satellites[0].Marketing.Listener.Addr()

		var group errgroup.Group
		for index, offer := range requests {
			o := offer
			id := strconv.Itoa(index + 1)

			group.Go(func() error {
				baseURL := "http://" + addr.String()

				req, err := http.PostForm(baseURL+o.Path, o.Values)
				if err != nil {
					return err
				}
				require.Equal(t, http.StatusOK, req.StatusCode)
				//reading out the rest of the connection
				_, err = io.Copy(ioutil.Discard, req.Body)
				if err != nil {
					return err
				}
				if err := req.Body.Close(); err != nil {
					return err
				}

				req, err = http.Get(baseURL)
				if err != nil {
					return err
				}
				require.Equal(t, http.StatusOK, req.StatusCode)
				_, err = io.Copy(ioutil.Discard, req.Body)
				if err != nil {
					return err
				}
				if err := req.Body.Close(); err != nil {
					return err
				}

				req, err = http.Post(baseURL+"/stop/"+id, "application/x-www-form-urlencoded", nil)
				if err != nil {
					return err
				}
				require.Equal(t, http.StatusOK, req.StatusCode)
				_, err = io.Copy(ioutil.Discard, req.Body)
				if err != nil {
					return err
				}
				if err := req.Body.Close(); err != nil {
					return err
				}

				return nil
			})
		}
		err := group.Wait()
		require.NoError(t, err)
	})
}

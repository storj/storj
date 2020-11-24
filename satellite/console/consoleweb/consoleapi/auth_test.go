// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
)

func TestAuth_Register(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		for i, test := range []struct {
			Partner      string
			ValidPartner bool
		}{
			{Partner: "minio", ValidPartner: true},
			{Partner: "Minio", ValidPartner: true},
			{Partner: "Raiden Network", ValidPartner: true},
			{Partner: "Raiden nEtwork", ValidPartner: true},
			{Partner: "invalid-name", ValidPartner: false},
		} {
			func() {
				registerData := struct {
					FullName       string `json:"fullName"`
					ShortName      string `json:"shortName"`
					Email          string `json:"email"`
					Partner        string `json:"partner"`
					PartnerID      string `json:"partnerId"`
					Password       string `json:"password"`
					SecretInput    string `json:"secret"`
					ReferrerUserID string `json:"referrerUserId"`
				}{
					FullName:  "testuser" + strconv.Itoa(i),
					ShortName: "test",
					Email:     "user@test" + strconv.Itoa(i),
					Partner:   test.Partner,
					Password:  "abc123",
				}

				jsonBody, err := json.Marshal(registerData)
				require.NoError(t, err)

				result, err := http.Post("http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/auth/register", "application/json", bytes.NewBuffer(jsonBody))
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, result.StatusCode)

				defer func() {
					err = result.Body.Close()
					require.NoError(t, err)
				}()

				body, err := ioutil.ReadAll(result.Body)
				require.NoError(t, err)

				var userID uuid.UUID
				err = json.Unmarshal(body, &userID)
				require.NoError(t, err)

				user, err := planet.Satellites[0].API.Console.Service.GetUser(ctx, userID)
				require.NoError(t, err)

				if test.ValidPartner {
					info, err := planet.Satellites[0].API.Marketing.PartnersService.ByName(ctx, test.Partner)
					require.NoError(t, err)
					require.Equal(t, info.UUID, user.PartnerID)
				} else {
					require.Equal(t, uuid.UUID{}, user.PartnerID)
				}
			}()
		}
	})
}

func TestDeleteAccount(t *testing.T) {
	// We do a black box testing because currently we don't allow to delete
	// accounts through the API hence we must always return an error response.

	config := &quick.Config{
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			// TODO: use or implement a better and thorough HTTP Request random generator

			var method string
			switch rnd.Intn(9) {
			case 0:
				method = http.MethodGet
			case 1:
				method = http.MethodHead
			case 2:
				method = http.MethodPost
			case 3:
				method = http.MethodPut
			case 4:
				method = http.MethodPatch
			case 5:
				method = http.MethodDelete
			case 6:
				method = http.MethodConnect
			case 7:
				method = http.MethodOptions
			case 8:
				method = http.MethodTrace
			default:
				t.Fatal("unexpected random value for HTTP method selection")
			}

			var path string
			{

				val, ok := quick.Value(reflect.TypeOf(""), rnd)
				require.True(t, ok, "quick.Values generator function couldn't generate a string")
				path = url.PathEscape(val.String())
			}

			var query string
			{
				nparams := rnd.Intn(27)
				params := make([]string, nparams)

				for i := 0; i < nparams; i++ {
					val, ok := quick.Value(reflect.TypeOf(""), rnd)
					require.True(t, ok, "quick.Values generator function couldn't generate a string")
					param := val.String()

					val, ok = quick.Value(reflect.TypeOf(""), rnd)
					require.True(t, ok, "quick.Values generator function couldn't generate a string")
					param += "=" + val.String()

					params[i] = param
				}

				query = url.QueryEscape(strings.Join(params, "&"))
			}

			var body io.Reader
			{
				val, ok := quick.Value(reflect.TypeOf([]byte(nil)), rnd)
				require.True(t, ok, "quick.Values generator function couldn't generate a byte slice")
				body = bytes.NewReader(val.Bytes())
			}

			withQuery := ""
			if len(query) > 0 {
				withQuery = "?"
			}

			reqURL, err := url.Parse("//storj.io/" + path + withQuery + query)
			require.NoError(t, err, "error when generating a random URL")
			req, err := http.NewRequest(method, reqURL.String(), body)
			require.NoError(t, err, "error when geneating a random request")
			values[0] = reflect.ValueOf(req)
		},
	}

	expectedHandler := func(_ *http.Request) (status int, body []byte) {
		return http.StatusNotImplemented, []byte("{\"error\":\"not implemented\"}\n")
	}

	actualHandler := func(r *http.Request) (status int, body []byte) {
		rr := httptest.NewRecorder()
		(&consoleapi.Auth{}).DeleteAccount(rr, r)

		//nolint:bodyclose
		result := rr.Result()
		defer func() {
			err := result.Body.Close()
			require.NoError(t, err)
		}()

		body, err := ioutil.ReadAll(result.Body)
		require.NoError(t, err)

		return result.StatusCode, body

	}

	err := quick.CheckEqual(expectedHandler, actualHandler, config)
	if err != nil {
		fmt.Printf("%+v\n", err)
		cerr := err.(*quick.CheckEqualError)

		t.Fatalf(`DeleteAccount handler has returned a different response:
round: %d
input args: %+v
expected response:
	status code: %d
	response body: %s
returned response:
	status code: %d
	response body: %s
`, cerr.Count, cerr.In, cerr.Out1[0], cerr.Out1[1], cerr.Out2[0], cerr.Out2[1])
	}
}

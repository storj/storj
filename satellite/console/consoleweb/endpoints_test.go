// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/apigen"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
)

func TestAuth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		user := test.defaultUser()

		{ // Register User
			_ = test.registerUser("user@mail.test", "#$Rnkl12i3nkljfds")
		}

		{ // Login_GetToken_Fail
			resp, body := test.request(
				http.MethodPost, "/auth/token",
				strings.NewReader(`{"email":"wrong@invalid.test","password":"wrong"}`))
			require.Nil(t, findCookie(resp, "_tokenKey"))
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			_ = body
			// TODO: require.Contains(t, body, "unauthorized")
		}

		{ // Login_GetToken_Pass
			test.login(user.email, user.password)
		}

		{ // Login_ChangePassword_IncorrectCurrentPassword
			resp, body := test.request(
				http.MethodPost, "/auth/account/change-password",
				test.toJSON(map[string]string{
					"email":       user.email,
					"password":    user.password + "1",
					"newPassword": user.password + "2",
				}))

			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			_ = body
			//TODO: require.Contains(t, body, "password was incorrect")
		}

		{ // Login_ChangePassword
			resp, _ := test.request(
				http.MethodPost, "/auth/account/change-password`",
				test.toJSON(map[string]string{
					"email":       user.email,
					"password":    user.password,
					"newPassword": user.password,
				}))
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		var oldCookies []*http.Cookie

		{ // Get_AccountInfo
			resp, body := test.request(http.MethodGet, "/auth/account", nil)
			require.Equal(test.t, http.StatusOK, resp.StatusCode)
			require.Contains(test.t, body, "fullName")
			oldCookies = resp.Cookies()

			var userIdentifier struct{ ID string }
			require.NoError(test.t, json.Unmarshal([]byte(body), &userIdentifier))
			require.NotEmpty(test.t, userIdentifier.ID)
		}

		{ // Update_AccountInfo
			newName := "new name"
			shortName := "NN"
			resp, _ := test.request(http.MethodPatch, "/auth/account", test.toJSON(map[string]string{
				"fullName":  newName,
				"shortName": shortName,
			}))
			require.Equal(test.t, http.StatusOK, resp.StatusCode)

			resp, body := test.request(http.MethodGet, "/auth/account", nil)
			require.Equal(test.t, http.StatusOK, resp.StatusCode)
			require.Contains(test.t, body, newName)
			require.Contains(test.t, body, shortName)

			// empty full name not allowed
			resp, _ = test.request(http.MethodPatch, "/auth/account", test.toJSON(map[string]string{
				"fullName":  "",
				"shortName": shortName,
			}))
			require.Equal(test.t, http.StatusBadRequest, resp.StatusCode)
		}

		{ // Get_FreezeStatus
			resp, body := test.request(http.MethodGet, "/auth/account/freezestatus", nil)
			require.Equal(test.t, http.StatusOK, resp.StatusCode)
			require.Contains(test.t, body, "frozen")
			require.Contains(test.t, body, "warned")

			var freezestatus struct {
				Frozen bool
				Warned bool
			}
			require.NoError(test.t, json.Unmarshal([]byte(body), &freezestatus))
			require.Equal(test.t, http.StatusOK, resp.StatusCode)
			require.False(test.t, freezestatus.Frozen)
			require.False(test.t, freezestatus.Warned)
		}

		{ // Test_UserSettings
			type expectedSettings struct {
				SessionDuration  *time.Duration
				OnboardingStart  bool
				OnboardingEnd    bool
				PassphrasePrompt bool
				OnboardingStep   *string
				NoticeDismissal  console.NoticeDismissal
			}
			testGetSettings := func(expected expectedSettings) {
				resp, body := test.request(http.MethodGet, "/auth/account/settings", nil)

				var settings struct {
					SessionDuration  *time.Duration
					OnboardingStart  bool
					OnboardingEnd    bool
					PassphrasePrompt bool
					OnboardingStep   *string
					NoticeDismissal  console.NoticeDismissal
				}
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.NoError(test.t, json.Unmarshal([]byte(body), &settings))
				require.Equal(test.t, expected.OnboardingStart, settings.OnboardingStart)
				require.Equal(test.t, expected.OnboardingEnd, settings.OnboardingEnd)
				require.Equal(test.t, expected.PassphrasePrompt, settings.PassphrasePrompt)
				require.Equal(test.t, expected.OnboardingStep, settings.OnboardingStep)
				require.Equal(test.t, expected.SessionDuration, settings.SessionDuration)
				require.Equal(test.t, expected.NoticeDismissal, settings.NoticeDismissal)
			}

			noticeDismissal := console.NoticeDismissal{
				FileGuide:                false,
				ServerSideEncryption:     false,
				PartnerUpgradeBanner:     false,
				ProjectMembersPassphrase: false,
				UploadOverwriteWarning:   false,
				VersioningBetaBanner:     false,
			}

			testGetSettings(expectedSettings{
				SessionDuration:  nil,
				OnboardingStart:  true,
				OnboardingEnd:    true,
				PassphrasePrompt: true,
				OnboardingStep:   nil,
				NoticeDismissal:  noticeDismissal,
			})

			step := "cli"
			duration := time.Duration(15) * time.Minute
			noticeDismissal.FileGuide = true
			noticeDismissal.ServerSideEncryption = true
			noticeDismissal.PartnerUpgradeBanner = true
			noticeDismissal.ProjectMembersPassphrase = true
			noticeDismissal.UploadOverwriteWarning = true
			noticeDismissal.VersioningBetaBanner = true
			resp, _ := test.request(http.MethodPatch, "/auth/account/settings",
				test.toJSON(map[string]interface{}{
					"sessionDuration":  duration,
					"onboardingStart":  true,
					"onboardingEnd":    false,
					"passphrasePrompt": false,
					"onboardingStep":   step,
					"noticeDismissal": map[string]bool{
						"fileGuide":                noticeDismissal.FileGuide,
						"serverSideEncryption":     noticeDismissal.ServerSideEncryption,
						"partnerUpgradeBanner":     noticeDismissal.PartnerUpgradeBanner,
						"projectMembersPassphrase": noticeDismissal.ProjectMembersPassphrase,
						"uploadOverwriteWarning":   noticeDismissal.UploadOverwriteWarning,
						"versioningBetaBanner":     noticeDismissal.VersioningBetaBanner,
					},
				}))

			require.Equal(t, http.StatusOK, resp.StatusCode)
			testGetSettings(expectedSettings{
				SessionDuration:  &duration,
				OnboardingStart:  true,
				OnboardingEnd:    false,
				PassphrasePrompt: false,
				OnboardingStep:   &step,
				NoticeDismissal:  noticeDismissal,
			})

			resp, _ = test.request(http.MethodPatch, "/auth/account/settings",
				test.toJSON(map[string]interface{}{
					"sessionDuration": nil,
					"onboardingStart": nil,
					"onboardingEnd":   nil,
					"onboardingStep":  nil,
					"noticeDismissal": nil,
				}))

			require.Equal(t, http.StatusOK, resp.StatusCode)
			// having passed nil to /auth/account/settings shouldn't have changed existing values.
			testGetSettings(expectedSettings{
				SessionDuration:  &duration,
				OnboardingStart:  true,
				OnboardingEnd:    false,
				PassphrasePrompt: false,
				OnboardingStep:   &step,
				NoticeDismissal:  noticeDismissal,
			})

			// having passed 0 as sessionDuration to /auth/account/settings should nullify it.
			resp, _ = test.request(http.MethodPatch, "/auth/account/settings",
				test.toJSON(map[string]interface{}{
					"sessionDuration": 0,
				}))

			require.Equal(t, http.StatusOK, resp.StatusCode)
			testGetSettings(expectedSettings{
				SessionDuration: nil,
				OnboardingStart: true,
				OnboardingEnd:   false,
				OnboardingStep:  &step,
				NoticeDismissal: noticeDismissal,
			})
		}

		{ // Logout
			resp, _ := test.request(http.MethodPost, "/auth/logout", nil)
			cookie := findCookie(resp, "_tokenKey")
			require.NotNil(t, cookie)
			require.Equal(t, "", cookie.Value)
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Get_AccountInfo shouldn't succeed after logging out
			resp, body := test.request(http.MethodGet, "/auth/account", nil)
			// TODO: wrong error text
			// require.Contains(test.t, body, "unauthorized")
			require.Contains(test.t, body, "error")
			require.Equal(test.t, http.StatusUnauthorized, resp.StatusCode)
		}

		{ // Get_AccountInfo shouldn't succeed with reused session cookie
			satURL, err := url.Parse(test.url(""))
			require.NoError(t, err)
			test.client.Jar.SetCookies(satURL, oldCookies)

			resp, body := test.request(http.MethodGet, "/auth/account", nil)
			require.Contains(test.t, body, "error")
			require.Equal(test.t, http.StatusUnauthorized, resp.StatusCode)
		}

		{ // repeated login attempts should end in too many requests
			hitRateLimiter := false
			for i := 0; i < 30; i++ {
				resp, _ := test.request(
					http.MethodPost, "/auth/token",
					strings.NewReader(`{"email":"wrong@invalid.test","password":"wrong"}`))
				require.Nil(t, findCookie(resp, "_tokenKey"))
				if resp.StatusCode != http.StatusUnauthorized {
					require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
					hitRateLimiter = true
					break
				}
			}
			require.True(t, hitRateLimiter, "did not hit rate limiter")
		}
	})
}

func TestAnalytics(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// test that sending /pageview event doesn't require authentication
		test := newTest(t, ctx, planet)
		user := test.defaultUser()

		{ // Register User
			_ = test.registerUser("user@mail.test", "#$Rnkl12i3nkljfds")
		}

		{ // Analytics_Pageview
			resp, _ := test.request(
				http.MethodPost, "/analytics/pageview",
				strings.NewReader(`{"url":"https://url.com/page","name":"pageview", "props": {"test": "test"}, "referrer": "storj.io"}`))
			require.Equal(t, http.StatusAccepted, resp.StatusCode)
		}

		{ // Login_GetToken_Pass
			test.login(user.email, user.password)
		}

		{ // Analytics_Pageview
			resp, _ := test.request(
				http.MethodPost, "/analytics/pageview",
				strings.NewReader(`{"url":"https://url.com/page","name":"pageview", "props": {"test": "test"}, "referrer": "storj.io"}`))
			require.Equal(t, http.StatusAccepted, resp.StatusCode)
		}
	})
}

func TestPayments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		user := test.defaultUser()

		{ // Unauthorized
			for _, path := range []string{
				"/payments/cards",
				"/payments/account/balance",
				"/payments/billing-history",
				"/payments/invoice-history",
				"/payments/account/charges?from=1619827200&to=1620844320",
			} {
				resp, body := test.request(http.MethodGet, path, nil)
				require.Contains(t, body, "unauthorized", path)
				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
			}
		}

		test.login(user.email, user.password)

		{ // Get_PaymentCards_EmptyReturn
			resp, body := test.request(http.MethodGet, "/payments/cards", nil)
			require.JSONEq(t, "[]", body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Get_AccountBalance
			resp, body := test.request(http.MethodGet, "/payments/account/balance", nil)
			require.Contains(t, body, "freeCredits")
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Get_BillingHistory
			resp, body := test.request(http.MethodGet, "/payments/billing-history", nil)
			require.JSONEq(t, "[]", body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Get_InvoiceHistory
			resp, body := test.request(http.MethodGet, "/payments/invoice-history?limit=1", nil)
			require.Contains(t, body, "items")
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Get_AccountChargesByDateRange
			resp, body := test.request(http.MethodGet, "/payments/account/charges?from=1619827200&to=1620844320", nil)
			require.Contains(t, body, "egress")
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Get_TaxCountries
			resp, body := test.request(http.MethodGet, "/payments/countries", nil)
			var countries []payments.TaxCountry
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &countries))
			require.Equal(t, payments.TaxCountries, countries)

			taxes := make([]payments.Tax, 0)
			for _, tax := range payments.Taxes {
				if tax.CountryCode == countries[0].Code {
					taxes = append(taxes, tax)
				}
			}
			resp, body = test.request(http.MethodGet, "/payments/countries/"+string(countries[0].Code)+"/taxes", nil)
			var txs []payments.Tax
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &txs))
			require.Equal(t, taxes, txs)
		}
	})
}

func TestWalletPayments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1, EnableSpanner: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		sat := planet.Satellites[0]

		userData := test.defaultUser()
		test.login(userData.email, userData.password)

		user, err := sat.DB.Console().Users().GetByEmail(ctx, userData.email)
		require.NoError(t, err)

		wallet := blockchaintest.NewAddress()
		err = sat.DB.Wallets().Add(ctx, user.ID, wallet)
		require.NoError(t, err)

		resp, _ := test.request(http.MethodGet, "/payments/wallet/payments", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestBuckets(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		user := test.defaultUser()

		{ // Unauthorized
			for _, path := range []string{
				"/buckets/bucket-names?projectID=" + test.defaultProjectID(),
			} {
				resp, body := test.request(http.MethodGet, path, nil)
				require.Contains(t, body, "unauthorized", path)
				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
			}
		}

		test.login(user.email, user.password)

		{ // Get_BucketNamesByProjectId
			resp, body := test.request(http.MethodGet, "/buckets/bucket-names?projectID="+test.defaultProjectID(), nil)
			// TODO: this should be []
			require.JSONEq(t, "null", body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // get bucket usages
			params := url.Values{
				"projectID": {test.defaultProjectID()},
				"before":    {time.Now().Add(time.Second).Format(apigen.DateFormat)},
				"limit":     {"1"},
				"search":    {""},
				"page":      {"1"},
			}

			resp, body := test.request(http.MethodGet, "/buckets/usage-totals?"+params.Encode(), nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			var page accounting.BucketUsagePage
			require.NoError(t, json.Unmarshal([]byte(body), &page))
			require.Empty(t, page.BucketUsages)

			const bucketName = "my-bucket"
			require.NoError(t, planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName))

			resp, body = test.request(http.MethodGet, "/buckets/usage-totals?"+params.Encode(), nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			require.NoError(t, json.Unmarshal([]byte(body), &page))
			require.NotEmpty(t, page.BucketUsages)
			require.Equal(t, bucketName, page.BucketUsages[0].BucketName)
		}
	})
}

func TestAPIKeys(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		user := test.defaultUser()
		test.login(user.email, user.password)

		{ // Get_ProjectAPIKeys
			var projects console.APIKeyPage
			path := "/api-keys/list-paged?projectID=" + test.defaultProjectID() + "&search=''&limit=6&page=1&order=1&orderDirection=1"
			resp, body := test.request(http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &projects))
			require.Contains(t, body, "apiKeys")
		}

		{ // Post_Create_APIKey
			var response console.CreateAPIKeyResponse
			path := "/api-keys/create/" + test.defaultProjectID()
			resp, body := test.request(http.MethodPost, path,
				test.toJSON(map[string]interface{}{
					"name": "testCreatedKey",
				}))
			require.Equal(t, http.StatusOK, resp.StatusCode)
			err := json.Unmarshal([]byte(body), &response)
			require.NoError(t, err)
			require.Contains(t, body, "key")
			require.Contains(t, body, "keyInfo")
			require.NotNil(t, response.KeyInfo)
		}

		{ // Delete_APIKeys_By_IDs
			var response *console.CreateAPIKeyResponse
			path := "/api-keys/create/" + test.defaultProjectID()
			resp, body := test.request(http.MethodPost, path,
				test.toJSON(map[string]interface{}{
					"name": "testCreatedKey1",
				}))
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &response))

			path = "/api-keys/delete-by-ids"
			resp, body = test.request(http.MethodDelete, path,
				test.toJSON(map[string]interface{}{
					"ids": []string{response.KeyInfo.ID.String()},
				}))
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Empty(t, body)
		}
	})
}

func TestProjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		user := test.defaultUser()
		test.login(user.email, user.password)

		{ // Get_Salt
			projectID := test.defaultProjectID()
			id, err := uuid.FromString(projectID)
			require.NoError(t, err)

			// get salt from endpoint
			var b64Salt string
			resp, body := test.request(http.MethodGet, fmt.Sprintf("/projects/%s/salt", test.defaultProjectID()), nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &b64Salt))

			// get salt from db and base64 encode it
			salt, err := planet.Satellites[0].DB.Console().Projects().GetSalt(ctx, id)
			require.NoError(t, err)
			require.Equal(t, b64Salt, base64.StdEncoding.EncodeToString(salt))
		}

		{ // Create_Project
			name := "a name"
			description := "a description"
			resp, body := test.request(http.MethodPost, "/projects", test.toJSON(map[string]interface{}{
				"name":        name,
				"description": description,
			}))
			require.Equal(t, http.StatusCreated, resp.StatusCode)

			var createdProject console.ProjectInfo
			require.NoError(t, json.Unmarshal([]byte(body), &createdProject))
			require.Equal(t, name, createdProject.Name)
			require.Equal(t, description, createdProject.Description)
		}

		{ // Get_User_Projects
			var projects []console.ProjectInfo
			resp, body := test.request(http.MethodGet, "/projects", nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &projects))
			require.NotEmpty(t, projects)
		}

		{ // Get_ProjectUsageLimitById
			resp, body := test.request(http.MethodGet, `/projects/`+test.defaultProjectID()+`/usage-limits`, nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Contains(t, body, "storageLimit")
		}

		{ // Get_OwnedProjects - HTTP
			var projects console.ProjectInfoPage
			resp, body := test.request(http.MethodGet, "/projects/paged?limit=6&page=1", nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(body), &projects))
			require.NotEmpty(t, projects.Projects)
		}

		{ // Post_ProjectRenameInvalid
			resp, body := test.request(http.MethodPatch, fmt.Sprintf("/projects/%s", test.defaultProjectID()),
				test.toJSON(map[string]interface{}{
					"name": "My Second Project with a long name",
				}))
			require.Contains(t, body, "error")
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		}

		{ // Post_ProjectRename
			resp, _ := test.request(http.MethodPatch, fmt.Sprintf("/projects/%s", test.defaultProjectID()),
				test.toJSON(map[string]interface{}{
					"name": "new name",
				}))
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}

		{ // Versioning_Opt_In
			projectID, err := uuid.FromString(test.defaultProjectID())
			require.NoError(t, err)

			checkVersioning := func(prompted bool, versioning console.DefaultVersioning) {
				project, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectID)
				require.NoError(t, err)

				require.Equal(t, prompted, project.PromptedForVersioningBeta)
				require.Equal(t, versioning, project.DefaultVersioning)
			}

			checkVersioning(false, console.Unversioned)

			resp, _ := test.request(http.MethodPatch, fmt.Sprintf("/projects/%s/versioning-opt-out", projectID), nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			checkVersioning(true, console.VersioningUnsupported)

			resp, _ = test.request(http.MethodPatch, fmt.Sprintf("/projects/%s/versioning-opt-in", projectID), nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			checkVersioning(true, console.Unversioned)
		}
	})
}

func TestWrongUser(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.RateLimit.Burst = 4
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		authorizedUser := test.defaultUser()
		unauthorizedUser := test.registerUser("user@mail.test", "#$Rnkl12i3nkljfds")
		if planet.Satellites[0].Config.Console.SignupActivationCodeEnabled {
			test.activateUser(ctx, unauthorizedUser.email)
		}

		type endpointTest struct {
			endpoint string
			method   string
			body     interface{}
		}

		baseProjectsUrl := "/projects"
		baseApiKeyUrl := "/api-keys"
		baseProjectIdUrl := fmt.Sprintf("%s/%s", baseProjectsUrl, test.defaultProjectID())
		getProjectResourceUrl := func(resource string) string {
			return fmt.Sprintf("%s/%s", baseProjectIdUrl, resource)
		}
		getIdAppendedApiKeyUrl := func(resource string) string {
			return fmt.Sprintf("%s/%s%s", baseApiKeyUrl, resource, test.defaultProjectID())
		}

		// login and create an api key and credit card to test deletion.
		test.login(authorizedUser.email, authorizedUser.password)
		resp, body := test.request(http.MethodPost, getIdAppendedApiKeyUrl("create/"), test.toJSON("some name"))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var response console.CreateAPIKeyResponse
		require.NoError(t, json.Unmarshal([]byte(body), &response))

		apiKeyId := response.KeyInfo.ID.String()

		test.login(unauthorizedUser.email, unauthorizedUser.password)

		testCases := []endpointTest{
			{
				endpoint: baseProjectIdUrl,
				method:   http.MethodPatch,
				body: map[string]interface{}{
					"name": "new name",
				},
			},
			{
				endpoint: getProjectResourceUrl("members") + "?emails=" + "some@email.test",
				method:   http.MethodDelete,
			},
			{
				endpoint: getProjectResourceUrl("salt"),
				method:   http.MethodGet,
			},
			{
				endpoint: getProjectResourceUrl("members"),
				method:   http.MethodGet,
			},
			{
				endpoint: getProjectResourceUrl("limit-increase"),
				method:   http.MethodPost,
				body: map[string]interface{}{
					"limitType":    "storage",
					"currentLimit": "100000000",
					"desiredLimit": "200000000",
				},
			},
			{
				endpoint: getProjectResourceUrl("invite") + "/" + "some@email.test",
				method:   http.MethodPost,
			},
			{
				endpoint: getProjectResourceUrl("usage-limits"),
				method:   http.MethodGet,
			},
			{
				endpoint: "/buckets/bucket-names?projectID=" + test.defaultProjectID(),
				method:   http.MethodGet,
			},
			{
				endpoint: "/buckets/usage-totals?limit=10&page=1&before=" + time.Now().Format(apigen.DateFormat) + "&projectID=" + test.defaultProjectID(),
				method:   http.MethodGet,
			},
			{
				endpoint: getProjectResourceUrl("daily-usage") + "?from=100000000&to=200000000000",
				method:   http.MethodGet,
			},
			{
				endpoint: getIdAppendedApiKeyUrl("create/"),
				method:   http.MethodPost,
				body:     "name",
			},
			{
				endpoint: getIdAppendedApiKeyUrl("delete-by-name?name=name&projectID="),
				method:   http.MethodDelete,
			},
			{
				endpoint: getIdAppendedApiKeyUrl("list-paged?limit=10&page=1&order=1&orderDirection=1&projectID="),
				method:   http.MethodGet,
			},
			{
				endpoint: getIdAppendedApiKeyUrl("api-key-names?projectID="),
				method:   http.MethodGet,
			},
			{
				endpoint: baseApiKeyUrl + "/delete-by-ids",
				method:   http.MethodDelete,
				body: map[string]interface{}{
					"ids": []string{apiKeyId},
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(fmt.Sprintf("Unauthorized on %s", testCase.endpoint), func(t *testing.T) {
				resp, body = test.request(testCase.method, testCase.endpoint, test.toJSON(testCase.body))
				require.Contains(t, body, "not authorized")
				require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			})
		}

		// login with correct user to make sure they have access.
		test.login(authorizedUser.email, authorizedUser.password)
		for _, testCase := range testCases {
			t.Run(fmt.Sprintf("Authorized on %s", testCase.endpoint), func(t *testing.T) {
				resp, body = test.request(testCase.method, testCase.endpoint, test.toJSON(testCase.body))
				require.NotContains(t, body, "not authorized")
				require.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
			})
		}
	})
}

type test struct {
	t      *testing.T
	ctx    *testcontext.Context
	planet *testplanet.Planet
	client *http.Client
}

func newTest(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) test {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return test{t: t, ctx: ctx, planet: planet, client: &http.Client{Jar: jar}}
}

type registeredUser struct {
	id       string
	email    string
	password string
}

func (test *test) request(method string, path string, data io.Reader) (resp Response, body string) {
	req, err := http.NewRequestWithContext(test.ctx, method, test.url(path), data)
	require.NoError(test.t, err)
	req.Header = map[string][]string{
		"sec-ch-ua":        {`" Not A;Brand";v="99"`, `"Chromium";v="90"`, `"Google Chrome";v="90"`},
		"sec-ch-ua-mobile": {"?0"},
		"User-Agent":       {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36"},
		"Content-Type":     {"application/json"},
		"Accept":           {"*/*"},
	}
	return test.do(req)
}

// Response is a wrapper for http.Request to prevent false-positive with bodyclose check.
type Response struct{ *http.Response }

func (test *test) do(req *http.Request) (_ Response, body string) {
	resp, err := test.client.Do(req)
	require.NoError(test.t, err)

	data, err := io.ReadAll(resp.Body)
	require.NoError(test.t, err)
	require.NoError(test.t, resp.Body.Close())

	return Response{resp}, string(data)
}

func (test *test) url(suffix string) string {
	return test.planet.Satellites[0].ConsoleURL() + "/api/v0" + suffix
}

func (test *test) toJSON(v interface{}) io.Reader {
	if str, ok := v.(string); ok {
		return strings.NewReader(str)
	}

	data, err := json.Marshal(v)
	require.NoError(test.t, err)
	return strings.NewReader(string(data))
}

func (test *test) defaultUser() registeredUser {
	user := test.planet.Uplinks[0].User[test.planet.Satellites[0].ID()]
	return registeredUser{
		email:    user.Email,
		password: user.Password,
	}
}

func (test *test) defaultProjectID() string { return test.planet.Uplinks[0].Projects[0].ID.String() }

func (test *test) login(email, password string) Response {
	resp, body := test.request(
		http.MethodPost, "/auth/token",
		test.toJSON(map[string]string{
			"email":    email,
			"password": password,
		}))
	require.Equal(test.t, http.StatusOK, resp.StatusCode)
	cookie := findCookie(resp, "_tokenKey")
	require.NotNil(test.t, cookie)

	var tokenInfo struct {
		Token string `json:"token"`
	}
	require.NoError(test.t, json.Unmarshal([]byte(body), &tokenInfo))
	require.Equal(test.t, http.StatusOK, resp.StatusCode)
	require.Equal(test.t, tokenInfo.Token, cookie.Value)

	return resp
}

func (test *test) registerUser(email, password string) registeredUser {
	resp, body := test.request(
		http.MethodPost, "/auth/register",
		test.toJSON(map[string]interface{}{
			"secret":           "",
			"password":         password,
			"fullName":         "Chester Cheeto",
			"shortName":        "",
			"email":            email,
			"partner":          "",
			"partnerId":        "",
			"isProfessional":   false,
			"position":         "",
			"companyName":      "",
			"employeeCount":    "",
			"haveSalesContact": false,
		}))

	require.Equal(test.t, http.StatusOK, resp.StatusCode)

	time.Sleep(time.Second) // TODO: hack-fix, register activates account asynchronously

	return registeredUser{
		id:       body,
		email:    email,
		password: password,
	}
}

func (test *test) activateUser(ctx context.Context, email string) {
	usersDB := test.planet.Satellites[0].DB.Console().Users()

	_, users, err := usersDB.GetByEmailWithUnverified(ctx, email)
	require.NoError(test.t, err)
	require.Len(test.t, users, 1)

	activeStatus := console.Active
	err = usersDB.Update(ctx, users[0].ID, console.UpdateUserRequest{Status: &activeStatus})
	require.NoError(test.t, err)
}

func findCookie(response Response, name string) *http.Cookie {
	for _, c := range response.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

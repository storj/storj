// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	admin "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

func TestAdmin_LicenseManagement(t *testing.T) {
	const licenseTestProductID = 42
	const usageOnlyTestProductID = 43
	const secondLicenseTestProductID = 44
	const licenseTestProductName = "Object Mount (Storj OS)"

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// The audit logger only registers its worker when enabled at startup.
				config.Admin.AuditLogger.Enabled = true

				usagePrice := paymentsconfig.ProjectUsagePrice{
					StorageTB: "1", EgressTB: "2", Segment: "3",
				}
				config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					licenseTestProductID: {
						Name:       licenseTestProductName,
						LicenseFee: "29.00",
					},
					usageOnlyTestProductID: {
						Name:              "Usage Only",
						ProjectUsagePrice: usagePrice,
					},
					secondLicenseTestProductID: {
						Name:       "Object Mount (Enterprise)",
						LicenseFee: "49.00",
					},
				})
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		entSvc := sat.API.Entitlements.Service
		consoleDB := sat.DB.Console()

		authInfo := &admin.AuthInfo{Email: "admin@storj.io", Groups: []string{"admin"}}

		paidExpiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
		freeExpiresAt := time.Now().Add(100 * 365 * 24 * time.Hour).UTC()

		newUser := func(t *testing.T, email string) uuid.UUID {
			user, err := sat.AddUser(ctx, console.CreateUser{FullName: "License Test User", Email: email}, 1)
			require.NoError(t, err)
			return user.ID
		}

		newProject := func(t *testing.T, ownerID uuid.UUID, name string) *console.Project {
			project, err := consoleDB.Projects().Insert(ctx, &console.Project{
				ID:      testrand.UUID(),
				Name:    name,
				OwnerID: ownerID,
			})
			require.NoError(t, err)
			return project
		}

		// newLicensedUser returns a user holding, in slice order: the free license every
		// account gets at signup, a revoked paid license, and an active paid twin of it.
		// Granting is only blocked by an active license, so grant/revoke/grant produces
		// exactly that pair, and it also shows that a paid license can sit alongside the
		// free one of the same type. Any mutation acting on the first field match would
		// touch the revoked twin instead of the billable license.
		newLicensedUser := func(t *testing.T, email string) uuid.UUID {
			userID := newUser(t, email)

			require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
				Type:      entitlements.OMLicenseType,
				Count:     2,
				ExpiresAt: freeExpiresAt,
				Reason:    "free seats",
			}).Err)

			paid := admin.GrantLicenseRequest{
				Type:      entitlements.OMLicenseType,
				ProductID: licenseTestProductID,
				Count:     3,
				ExpiresAt: paidExpiresAt,
				Reason:    "paid seats",
			}
			require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, paid).Err)
			require.NoError(t, service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
				Type:      paid.Type,
				ProductID: paid.ProductID,
				ExpiresAt: paid.ExpiresAt,
				Reason:    "revoke the first",
			}).Err)
			paid.Reason = "re-grant after revocation"
			require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, paid).Err)

			licenses, err := entSvc.Licenses().Get(ctx, userID)
			require.NoError(t, err)
			require.Len(t, licenses.Licenses, 3)
			require.True(t, licenses.Licenses[0].RevokedAt.IsZero(), "the free license must stay active")
			require.False(t, licenses.Licenses[1].RevokedAt.IsZero(), "the revoked twin must come before the active one")
			require.True(t, licenses.Licenses[2].RevokedAt.IsZero())

			return userID
		}

		// The active paid license held by a newLicensedUser.
		const licenseType, productID = entitlements.OMLicenseType, uint(licenseTestProductID)

		t.Run("get", func(t *testing.T) {
			t.Run("user without licenses", func(t *testing.T) {
				resp, apiErr := service.GetUserLicenses(ctx, newUser(t, "get-empty@storj.io"))
				require.NoError(t, apiErr.Err)
				require.Empty(t, resp.Licenses)
			})

			t.Run("unknown user", func(t *testing.T) {
				_, apiErr := service.GetUserLicenses(ctx, testrand.UUID())
				require.Equal(t, http.StatusNotFound, apiErr.Status)
			})

			t.Run("names the product and omits an unrecorded start time", func(t *testing.T) {
				userID := newUser(t, "get-free@storj.io")
				// Written directly: a license predating StartsAt cannot be granted
				// through the API anymore.
				require.NoError(t, entSvc.Licenses().Set(ctx, userID, entitlements.AccountLicenses{
					Licenses: []entitlements.AccountLicense{{
						Type:      entitlements.OMLicenseType,
						Count:     2,
						ExpiresAt: freeExpiresAt,
					}},
				}))

				resp, apiErr := service.GetUserLicenses(ctx, userID)
				require.NoError(t, apiErr.Err)
				require.Len(t, resp.Licenses, 1)
				require.Zero(t, resp.Licenses[0].ProductID)
				require.Equal(t, "Free", resp.Licenses[0].ProductName)
				require.Equal(t, 2, resp.Licenses[0].Count)
				require.Nil(t, resp.Licenses[0].StartsAt, "a zero start time must not be reported as a date")
			})
		})

		t.Run("grant", func(t *testing.T) {
			t.Run("records every field and surfaces it", func(t *testing.T) {
				userID := newUser(t, "grant-fields@storj.io")
				project := newProject(t, userID, "grant-fields-project")

				before := time.Now().UTC()
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:       entitlements.OMLicenseType,
					ProductID:  licenseTestProductID,
					Count:      5,
					PublicId:   project.PublicID.String(),
					BucketName: "test-bucket",
					ExpiresAt:  paidExpiresAt,
					Key:        "test-key-value",
					Reason:     "grant with every field",
				}).Err)

				resp, apiErr := service.GetUserLicenses(ctx, userID)
				require.NoError(t, apiErr.Err)
				require.Len(t, resp.Licenses, 1)

				got := resp.Licenses[0]
				require.Equal(t, entitlements.OMLicenseType, got.Type)
				require.Equal(t, uint(licenseTestProductID), got.ProductID)
				require.Equal(t, licenseTestProductName, got.ProductName)
				require.Equal(t, 5, got.Count)
				require.Equal(t, project.PublicID.String(), got.PublicId)
				require.Equal(t, "test-bucket", got.BucketName)
				require.Equal(t, "test-key-value", got.Key)
				require.WithinDuration(t, paidExpiresAt, got.ExpiresAt, time.Second)
				require.Nil(t, got.RevokedAt)
				// Billing prorates the first period from StartsAt.
				require.NotNil(t, got.StartsAt)
				require.False(t, got.StartsAt.Before(before.Add(-time.Second)))
			})

			t.Run("stores the canonical public ID", func(t *testing.T) {
				userID := newUser(t, "grant-canonical@storj.io")
				project := newProject(t, userID, "grant-canonical-project")

				// GetActive matches on UUID.String(), so any other spelling that
				// uuid.FromString accepts would be billed without ever applying.
				grant := admin.GrantLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					Count:     1,
					PublicId:  strings.ReplaceAll(project.PublicID.String(), "-", ""),
					ExpiresAt: paidExpiresAt,
					Reason:    "grant with an undashed public ID",
				}
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, grant).Err)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Equal(t, project.PublicID.String(), licenses.Licenses[0].PublicID)

				now := time.Now()
				active, err := entSvc.Licenses().GetActive(ctx, userID, entitlements.GetActiveOptions{
					LicenseType: licenseType, PublicID: project.PublicID, Now: &now,
				})
				require.NoError(t, err)
				require.Len(t, active, 1, "the license must be in force for its project")

				grant.Reason = "duplicate scope"
				require.Equal(t, http.StatusConflict,
					service.GrantUserLicense(ctx, authInfo, userID, grant).Status)

				require.NoError(t, service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
					Type: licenseType, ProductID: productID, PublicId: grant.PublicId,
					ExpiresAt: paidExpiresAt, Reason: "revoke with an undashed public ID",
				}).Err)
			})

			t.Run("matches a scope stored before grants normalized it", func(t *testing.T) {
				userID := newUser(t, "legacy-scope@storj.io")
				project := newProject(t, userID, "legacy-scope-project")
				canonical := project.PublicID.String()

				legacy := entitlements.AccountLicense{
					Type:      licenseType,
					ProductID: productID,
					Count:     1,
					PublicID:  strings.ReplaceAll(canonical, "-", ""),
					ExpiresAt: paidExpiresAt,
				}

				grant := func(product uint) api.HTTPError {
					return service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
						Type: licenseType, ProductID: product, Count: 1, PublicId: canonical,
						ExpiresAt: paidExpiresAt, Reason: "duplicate of a legacy scope",
					})
				}

				// A free legacy scope only reaches the same-identity branch of the
				// conflict check, which the paid cases return before.
				for _, tt := range []struct {
					name    string
					product uint
					status  int
					call    func(product uint) api.HTTPError
				}{{
					name:    "a paid grant conflicts with it",
					product: productID,
					status:  http.StatusConflict,
					call:    grant,
				}, {
					name:    "a free grant conflicts with a free one",
					product: 0,
					status:  http.StatusConflict,
					call:    grant,
				}, {
					name:    "revoke reaches it",
					product: productID,
					call: func(product uint) api.HTTPError {
						return service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
							Type: licenseType, ProductID: product, PublicId: canonical,
							ExpiresAt: paidExpiresAt, Reason: "revoke a legacy scope",
						})
					},
				}, {
					name:    "update reaches it",
					product: productID,
					call: func(product uint) api.HTTPError {
						return service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
							Type: licenseType, ProductID: product, PublicId: canonical,
							ExpiresAt: paidExpiresAt, NewExpiresAt: paidExpiresAt.AddDate(0, 1, 0),
							Reason: "extend a legacy scope",
						})
					},
				}, {
					name:    "delete reaches it",
					product: productID,
					call: func(product uint) api.HTTPError {
						return service.DeleteUserLicense(ctx, authInfo, userID, admin.DeleteLicenseRequest{
							Type: licenseType, ProductID: product, PublicId: canonical,
							ExpiresAt: paidExpiresAt, Reason: "delete a legacy scope",
						})
					},
				}} {
					t.Run(tt.name, func(t *testing.T) {
						stored := legacy
						stored.ProductID = tt.product
						require.NoError(t, entSvc.Licenses().Set(ctx, userID,
							entitlements.AccountLicenses{Licenses: []entitlements.AccountLicense{stored}}))
						require.Equal(t, tt.status, tt.call(tt.product).Status)
					})
				}
			})

			t.Run("rejects invalid requests", func(t *testing.T) {
				userID := newUser(t, "grant-validation@storj.io")

				for _, tt := range []struct {
					name     string
					request  admin.GrantLicenseRequest
					status   int
					contains string
				}{
					{
						name:    "missing expiration",
						request: admin.GrantLicenseRequest{Type: "a-license", Count: 1, Reason: "missing expiration"},
						status:  http.StatusBadRequest,
					}, {
						name:    "past expiration",
						request: admin.GrantLicenseRequest{Type: "a-license", Count: 1, ExpiresAt: time.Now().Add(-time.Hour), Reason: "past expiration"},
						status:  http.StatusBadRequest,
					}, {
						name:    "zero seat count",
						request: admin.GrantLicenseRequest{Type: "a-license", ExpiresAt: paidExpiresAt, Reason: "zero count"},
						status:  http.StatusBadRequest,
					}, {
						name:    "negative seat count",
						request: admin.GrantLicenseRequest{Type: "a-license", Count: -5, ExpiresAt: paidExpiresAt, Reason: "negative count"},
						status:  http.StatusBadRequest,
					}, {
						name:    "malformed project ID",
						request: admin.GrantLicenseRequest{Type: "a-license", Count: 1, PublicId: "not-a-uuid", ExpiresAt: paidExpiresAt, Reason: "malformed project"},
						status:  http.StatusBadRequest,
					}, {
						name:    "unknown project",
						request: admin.GrantLicenseRequest{Type: "a-license", Count: 1, PublicId: uuid.UUID{}.String(), ExpiresAt: paidExpiresAt, Reason: "unknown project"},
						status:  http.StatusNotFound,
					}, {
						name:    "unknown product",
						request: admin.GrantLicenseRequest{Type: "a-license", ProductID: 9999, Count: 1, ExpiresAt: paidExpiresAt, Reason: "unknown product"},
						status:  http.StatusBadRequest,
					}, {
						name:     "product without a license fee",
						request:  admin.GrantLicenseRequest{Type: "a-license", ProductID: usageOnlyTestProductID, Count: 1, ExpiresAt: paidExpiresAt, Reason: "usage-only product"},
						status:   http.StatusBadRequest,
						contains: "has no license fee",
					}, {
						// Truncating this to int32 gives licenseTestProductID, a configured
						// product, so without a range check it would validate and then be
						// stored, billed and displayed as a different product.
						name:    "product ID wider than int32",
						request: admin.GrantLicenseRequest{Type: "a-license", ProductID: uint(math.MaxUint32) + 1 + licenseTestProductID, Count: 1, ExpiresAt: paidExpiresAt, Reason: "wide product id"},
						status:  http.StatusBadRequest,
					},
				} {
					t.Run(tt.name, func(t *testing.T) {
						apiErr := service.GrantUserLicense(ctx, authInfo, userID, tt.request)
						require.Error(t, apiErr.Err)
						require.Equal(t, tt.status, apiErr.Status)
						if tt.contains != "" {
							require.Contains(t, apiErr.Err.Error(), tt.contains)
						}
					})
				}

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Empty(t, licenses.Licenses, "a rejected grant must not persist anything")
			})

			t.Run("conflicts only with an active license of the same identity", func(t *testing.T) {
				// The fixture already grants a paid license alongside the free one of
				// the same type, and re-grants it once the first was revoked.
				userID := newLicensedUser(t, "grant-conflict@storj.io")

				// Basic validation runs before the scope is consulted, so a malformed
				// request is reported as such rather than as a conflict.
				require.Equal(t, http.StatusBadRequest,
					service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
						Type: licenseType, ProductID: productID, ExpiresAt: paidExpiresAt,
						Reason: "zero count on a scope that is already taken",
					}).Status)

				// A different expiry does not make it a different license.
				apiErr := service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseTestProductID,
					Count:     1,
					ExpiresAt: paidExpiresAt.Add(24 * time.Hour),
					Reason:    "duplicate",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusConflict, apiErr.Status)

				// Another paid product for the same type and scope conflicts too, so
				// that an upgrade cannot leave both products billing seats.
				apiErr = service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:      entitlements.OMLicenseType,
					ProductID: secondLicenseTestProductID,
					Count:     1,
					ExpiresAt: paidExpiresAt,
					Reason:    "upgrade without revoking first",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusConflict, apiErr.Status)

				// The fixture's paid license is account-wide, and an empty scope is a
				// wildcard, so a bucket-scoped grant would bill the same traffic twice.
				apiErr = service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:       entitlements.OMLicenseType,
					ProductID:  licenseTestProductID,
					Count:      1,
					BucketName: "another-bucket",
					ExpiresAt:  paidExpiresAt,
					Reason:     "bucket seats under an account-wide license",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusConflict, apiErr.Status)

				resp, listErr := service.GetUserLicenses(ctx, userID)
				require.NoError(t, listErr.Err)
				require.Len(t, resp.Licenses, 3, "no conflicting grant may have landed")
			})

			t.Run("scopes that cannot overlap do not conflict", func(t *testing.T) {
				userID := newUser(t, "grant-scopes@storj.io")

				// Disjoint buckets never bill for each other's traffic.
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:       entitlements.OMLicenseType,
					ProductID:  licenseTestProductID,
					Count:      1,
					BucketName: "one-bucket",
					ExpiresAt:  paidExpiresAt,
					Reason:     "seats for one bucket",
				}).Err)
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:       entitlements.OMLicenseType,
					ProductID:  secondLicenseTestProductID,
					Count:      1,
					BucketName: "another-bucket",
					ExpiresAt:  paidExpiresAt,
					Reason:     "seats for the other product",
				}).Err)

				// A free license bills nothing, so a narrower one is not refused.
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:      entitlements.OMLicenseType,
					Count:     2,
					ExpiresAt: freeExpiresAt,
					Reason:    "account-wide free seats",
				}).Err)
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:       entitlements.OMLicenseType,
					Count:      2,
					BucketName: "one-bucket",
					ExpiresAt:  freeExpiresAt,
					Reason:     "free seats for one bucket",
				}).Err)

				resp, listErr := service.GetUserLicenses(ctx, userID)
				require.NoError(t, listErr.Err)
				require.Len(t, resp.Licenses, 4)
			})

			t.Run("conflicts with a license that never expires", func(t *testing.T) {
				userID := newUser(t, "grant-perpetual@storj.io")

				// A zero ExpiresAt never expires and is billed for the whole period, so
				// the scan has to read it as in force rather than as long expired.
				require.NoError(t, entSvc.Licenses().Set(ctx, userID, entitlements.AccountLicenses{
					Licenses: []entitlements.AccountLicense{{
						Type:      entitlements.OMLicenseType,
						ProductID: licenseTestProductID,
						Count:     4,
						StartsAt:  time.Now().Add(-24 * time.Hour).UTC(),
					}},
				}))

				apiErr := service.GrantUserLicense(ctx, authInfo, userID, admin.GrantLicenseRequest{
					Type:      entitlements.OMLicenseType,
					ProductID: licenseTestProductID,
					Count:     1,
					ExpiresAt: paidExpiresAt,
					Reason:    "second license over a perpetual one",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusConflict, apiErr.Status)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 1)
			})
		})

		t.Run("revoke", func(t *testing.T) {
			t.Run("stamps the active license and leaves the rest alone", func(t *testing.T) {
				userID := newLicensedUser(t, "revoke-active@storj.io")

				require.NoError(t, service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: paidExpiresAt,
					Reason:    "revoke the live one",
				}).Err)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 3)
				for i, license := range licenses.Licenses {
					if license.ProductID == 0 {
						require.True(t, license.RevokedAt.IsZero(), "the free license must not be revoked")
					} else {
						require.False(t, license.RevokedAt.IsZero(), "paid license %d is still billable", i)
					}
				}

				// Nothing active is left, so revoking again is refused rather than
				// silently re-stamping a dead entry.
				apiErr := service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: paidExpiresAt,
					Reason:    "revoke once more",
				})
				require.Equal(t, http.StatusBadRequest, apiErr.Status)
				require.Contains(t, apiErr.Err.Error(), "already revoked")
			})

			t.Run("does not reach a license of another product", func(t *testing.T) {
				userID := newLicensedUser(t, "revoke-wrong-product@storj.io")

				// The free license expires at freeExpiresAt, so this pairs a product
				// with an expiry that belongs to a different license.
				apiErr := service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: freeExpiresAt,
					Reason:    "wrong product",
				})
				require.Equal(t, http.StatusNotFound, apiErr.Status)
			})
		})

		t.Run("delete", func(t *testing.T) {
			t.Run("removes the license revoked at the given time", func(t *testing.T) {
				userID := newLicensedUser(t, "delete-revoked@storj.io")

				before, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				revokedAt := before.Licenses[1].RevokedAt
				require.False(t, revokedAt.IsZero())

				require.NoError(t, service.DeleteUserLicense(ctx, authInfo, userID, admin.DeleteLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: paidExpiresAt,
					RevokedAt: &revokedAt,
					Reason:    "clean up the revoked one",
				}).Err)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 2)
				for _, license := range licenses.Licenses {
					require.True(t, license.RevokedAt.IsZero(), "the billable license must survive")
				}

				// A revocation time that matches nothing must not fall back to the
				// active license.
				missing := revokedAt.Add(-time.Hour)
				apiErr := service.DeleteUserLicense(ctx, authInfo, userID, admin.DeleteLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: paidExpiresAt,
					RevokedAt: &missing,
					Reason:    "delete a license that is not there",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusNotFound, apiErr.Status)

				licenses, err = entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 2)
			})

			t.Run("without a revocation time removes the active license first", func(t *testing.T) {
				userID := newLicensedUser(t, "delete-active@storj.io")

				request := admin.DeleteLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: paidExpiresAt,
					Reason:    "delete the live one",
				}
				require.NoError(t, service.DeleteUserLicense(ctx, authInfo, userID, request).Err)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 2)
				for _, license := range licenses.Licenses {
					if license.ProductID != 0 {
						require.False(t, license.RevokedAt.IsZero(), "the active license should have gone")
					}
				}

				// The revoked entry can then be cleaned up.
				request.Reason = "clean up the revoked one"
				require.NoError(t, service.DeleteUserLicense(ctx, authInfo, userID, request).Err)

				licenses, err = entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 1)
				require.Zero(t, licenses.Licenses[0].ProductID, "the free license must survive")
			})
		})

		t.Run("update", func(t *testing.T) {
			t.Run("extends the active license and touches nothing else", func(t *testing.T) {
				userID := newLicensedUser(t, "update-active@storj.io")

				newExpiresAt := paidExpiresAt.AddDate(0, 1, 0)
				require.NoError(t, service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
					Type:         licenseType,
					ProductID:    productID,
					ExpiresAt:    paidExpiresAt,
					NewExpiresAt: newExpiresAt,
					Reason:       "extend the live one",
				}).Err)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 3)
				for _, license := range licenses.Licenses {
					switch {
					case license.ProductID == 0:
						require.WithinDuration(t, freeExpiresAt, license.ExpiresAt, time.Second)
					case license.RevokedAt.IsZero():
						require.WithinDuration(t, newExpiresAt, license.ExpiresAt, time.Second)
					default:
						require.WithinDuration(t, paidExpiresAt, license.ExpiresAt, time.Second, "the revoked twin must be untouched")
					}
				}
			})

			t.Run("shortening leaves the other fields as they were", func(t *testing.T) {
				userID := newUser(t, "update-fields@storj.io")
				project := newProject(t, userID, "update-fields-project")

				grant := admin.GrantLicenseRequest{
					Type:       entitlements.OMLicenseType,
					ProductID:  licenseTestProductID,
					Count:      5,
					PublicId:   project.PublicID.String(),
					BucketName: "test-bucket",
					ExpiresAt:  paidExpiresAt,
					Key:        "test-key-value",
					Reason:     "grant for update",
				}
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, grant).Err)

				newExpiresAt := time.Now().Add(7 * 24 * time.Hour).UTC()
				require.NoError(t, service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
					Type:         grant.Type,
					ProductID:    grant.ProductID,
					PublicId:     grant.PublicId,
					BucketName:   grant.BucketName,
					ExpiresAt:    grant.ExpiresAt,
					NewExpiresAt: newExpiresAt,
					Reason:       "shortening license duration",
				}).Err)

				resp, apiErr := service.GetUserLicenses(ctx, userID)
				require.NoError(t, apiErr.Err)
				require.Len(t, resp.Licenses, 1)

				updated := resp.Licenses[0]
				require.WithinDuration(t, newExpiresAt, updated.ExpiresAt, time.Second)
				require.Equal(t, grant.Type, updated.Type)
				require.Equal(t, grant.ProductID, updated.ProductID)
				require.Equal(t, grant.Count, updated.Count)
				require.Equal(t, grant.PublicId, updated.PublicId)
				require.Equal(t, grant.BucketName, updated.BucketName)
				require.Equal(t, grant.Key, updated.Key)
				require.Nil(t, updated.RevokedAt)
			})

			t.Run("rejects invalid requests", func(t *testing.T) {
				userID := newLicensedUser(t, "update-validation@storj.io")

				for _, tt := range []struct {
					name     string
					request  admin.UpdateLicenseRequest
					status   int
					contains string
				}{
					{
						name:    "missing new expiration",
						request: admin.UpdateLicenseRequest{Type: licenseType, ProductID: productID, ExpiresAt: paidExpiresAt, Reason: "missing new expiration"},
						status:  http.StatusBadRequest,
					}, {
						name:    "new expiration in the past",
						request: admin.UpdateLicenseRequest{Type: licenseType, ProductID: productID, ExpiresAt: paidExpiresAt, NewExpiresAt: time.Now().Add(-time.Hour), Reason: "past date"},
						status:  http.StatusBadRequest,
					}, {
						name:    "current expiration does not match",
						request: admin.UpdateLicenseRequest{Type: licenseType, ProductID: productID, ExpiresAt: paidExpiresAt.Add(24 * time.Hour), NewExpiresAt: paidExpiresAt.Add(48 * time.Hour), Reason: "wrong expiry"},
						status:  http.StatusNotFound,
					},
				} {
					t.Run(tt.name, func(t *testing.T) {
						apiErr := service.UpdateUserLicense(ctx, authInfo, userID, tt.request)
						require.Error(t, apiErr.Err)
						require.Equal(t, tt.status, apiErr.Status)
						if tt.contains != "" {
							require.Contains(t, apiErr.Err.Error(), tt.contains)
						}
					})
				}
			})

			t.Run("refuses a license that is only revoked", func(t *testing.T) {
				userID := newLicensedUser(t, "update-revoked@storj.io")

				require.NoError(t, service.RevokeUserLicense(ctx, authInfo, userID, admin.RevokeLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					ExpiresAt: paidExpiresAt,
					Reason:    "revoke the live one",
				}).Err)

				apiErr := service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
					Type:         licenseType,
					ProductID:    productID,
					ExpiresAt:    paidExpiresAt,
					NewExpiresAt: paidExpiresAt.AddDate(0, 1, 0),
					Reason:       "extend a revoked license",
				})
				require.Equal(t, http.StatusBadRequest, apiErr.Status)
				require.Contains(t, apiErr.Err.Error(), "revoked")
			})

			t.Run("refuses a license whose term has already ended", func(t *testing.T) {
				userID := newUser(t, "update-expired@storj.io")

				firstExpiresAt := time.Now().Add(time.Hour).UTC()
				first := admin.GrantLicenseRequest{
					Type:      licenseType,
					ProductID: productID,
					Count:     2,
					ExpiresAt: firstExpiresAt,
					Reason:    "first term",
				}
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, first).Err)

				// Move past the first term. Moving its expiry would keep the StartsAt of
				// the term that ended, so the whole gap would be billed.
				service.TestSetNowFn(func() time.Time { return firstExpiresAt.Add(time.Hour) })
				defer service.TestSetNowFn(time.Now)

				apiErr := service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
					Type:         licenseType,
					ProductID:    productID,
					ExpiresAt:    firstExpiresAt,
					NewExpiresAt: paidExpiresAt,
					Reason:       "extend a lapsed term",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusBadRequest, apiErr.Status)
				require.Contains(t, apiErr.Err.Error(), "expired")

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.Len(t, licenses.Licenses, 1)
				require.WithinDuration(t, firstExpiresAt, licenses.Licenses[0].ExpiresAt, time.Second,
					"the lapsed term must be left as it was")

				// Granting is the way to renew; the lapsed term does not block it.
				second := first
				second.Count = 3
				second.ExpiresAt = paidExpiresAt
				second.Reason = "second term"
				require.NoError(t, service.GrantUserLicense(ctx, authInfo, userID, second).Err)

				// The term actually in force can still be extended.
				require.NoError(t, service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
					Type:         licenseType,
					ProductID:    productID,
					ExpiresAt:    paidExpiresAt,
					NewExpiresAt: paidExpiresAt.AddDate(0, 1, 0),
					Reason:       "extend the second term",
				}).Err)
			})

			t.Run("refuses to prolong an overlap that predates the conflict check", func(t *testing.T) {
				userID := newUser(t, "update-overlap@storj.io")

				// Stored data from before the conflict check can hold two licenses in
				// force over one scope. Extending either prolongs the overlap.
				shorter := time.Now().Add(48 * time.Hour).UTC()
				twin := entitlements.AccountLicense{
					Type:      licenseType,
					ProductID: productID,
					Count:     2,
					StartsAt:  time.Now().Add(-24 * time.Hour).UTC(),
					ExpiresAt: shorter,
				}
				longer := twin
				longer.ExpiresAt = paidExpiresAt
				require.NoError(t, entSvc.Licenses().Set(ctx, userID, entitlements.AccountLicenses{
					Licenses: []entitlements.AccountLicense{twin, longer},
				}))

				apiErr := service.UpdateUserLicense(ctx, authInfo, userID, admin.UpdateLicenseRequest{
					Type:         licenseType,
					ProductID:    productID,
					ExpiresAt:    shorter,
					NewExpiresAt: paidExpiresAt.AddDate(0, 1, 0),
					Reason:       "extend one of two overlapping licenses",
				})
				require.Error(t, apiErr.Err)
				require.Equal(t, http.StatusConflict, apiErr.Status)

				licenses, err := entSvc.Licenses().Get(ctx, userID)
				require.NoError(t, err)
				require.WithinDuration(t, shorter, licenses.Licenses[0].ExpiresAt, time.Second)
			})
		})

		// The guards below are identical across the mutating endpoints, so they are
		// asserted once for all of them rather than per endpoint.
		t.Run("mutation guards", func(t *testing.T) {
			type mutation struct {
				name string
				// targetsExisting is false for grant, which creates a license rather
				// than looking one up.
				targetsExisting bool
				call            func(auth *admin.AuthInfo, userID uuid.UUID, licenseType, reason string) api.HTTPError
			}

			mutations := []mutation{{
				name: "grant",
				call: func(auth *admin.AuthInfo, userID uuid.UUID, licenseType, reason string) api.HTTPError {
					return service.GrantUserLicense(ctx, auth, userID, admin.GrantLicenseRequest{
						Type: licenseType, Count: 1, ExpiresAt: paidExpiresAt, Reason: reason,
					})
				},
			}, {
				name:            "revoke",
				targetsExisting: true,
				call: func(auth *admin.AuthInfo, userID uuid.UUID, licenseType, reason string) api.HTTPError {
					return service.RevokeUserLicense(ctx, auth, userID, admin.RevokeLicenseRequest{
						Type: licenseType, ExpiresAt: paidExpiresAt, Reason: reason,
					})
				},
			}, {
				name:            "delete",
				targetsExisting: true,
				call: func(auth *admin.AuthInfo, userID uuid.UUID, licenseType, reason string) api.HTTPError {
					return service.DeleteUserLicense(ctx, auth, userID, admin.DeleteLicenseRequest{
						Type: licenseType, ExpiresAt: paidExpiresAt, Reason: reason,
					})
				},
			}, {
				name:            "update",
				targetsExisting: true,
				call: func(auth *admin.AuthInfo, userID uuid.UUID, licenseType, reason string) api.HTTPError {
					return service.UpdateUserLicense(ctx, auth, userID, admin.UpdateLicenseRequest{
						Type: licenseType, ExpiresAt: paidExpiresAt,
						NewExpiresAt: paidExpiresAt.Add(24 * time.Hour), Reason: reason,
					})
				},
			}}

			userID := newUser(t, "mutation-guards@storj.io")
			unauthorized := &admin.AuthInfo{Email: "admin@storj.io", Groups: []string{}}

			for _, m := range mutations {
				t.Run(m.name, func(t *testing.T) {
					require.Equal(t, http.StatusUnauthorized,
						m.call(nil, userID, "guarded-license", "no auth info").Status)
					require.Equal(t, http.StatusUnauthorized,
						m.call(unauthorized, userID, "guarded-license", "no groups").Status)
					require.Equal(t, http.StatusBadRequest,
						m.call(authInfo, userID, "guarded-license", "").Status, "a reason is required")
					require.Equal(t, http.StatusBadRequest,
						m.call(authInfo, userID, "", "missing type").Status, "a license type is required")
					require.Equal(t, http.StatusNotFound,
						m.call(authInfo, testrand.UUID(), "guarded-license", "unknown user").Status)

					if m.targetsExisting {
						require.Equal(t, http.StatusNotFound,
							m.call(authInfo, userID, "guarded-license", "unknown license").Status)
					}
				})
			}

			t.Run("tenant-scoped admin has no access at all", func(t *testing.T) {
				tenantID := "tenant-a"
				service.TestSetTenantID(&tenantID)
				defer service.TestSetTenantID(nil)

				_, apiErr := service.GetUserLicenses(ctx, userID)
				require.Equal(t, http.StatusForbidden, apiErr.Status)
				for _, m := range mutations {
					require.Equal(t, http.StatusForbidden,
						m.call(authInfo, userID, "guarded-license", "tenant-scoped").Status, m.name)
				}
			})

			licenses, err := entSvc.Licenses().Get(ctx, userID)
			require.NoError(t, err)
			require.Empty(t, licenses.Licenses, "a refused mutation must not persist anything")
		})
	})
}

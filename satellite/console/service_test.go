// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	stripeLib "github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/currency"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/httpmock"
	"storj.io/storj/private/post"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/valdi/valdiclient"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/tenancy"
	"storj.io/uplink/private/metaclient"
)

func TestService(t *testing.T) {
	placements := make(map[int]string)
	for i := 0; i < 4; i++ {
		placements[i] = fmt.Sprintf("loc-%d", i)
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 4,
		Timeout: -1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.StripeFreeTierCouponID = stripe.MockCouponID1
				var plcStr string
				for k, v := range placements {
					plcStr += fmt.Sprintf(`%d:annotation("location", "%s"); `, k, v)
				}
				config.Placement = nodeselection.ConfigurablePlacementRule{PlacementRules: plcStr}
				config.Console.VarPartners = []string{"partner1"}
				config.Console.DeleteProjectEnabled = true
				config.Metainfo.UseBucketLevelObjectVersioning = true
				config.Entitlements.Enabled = true
				config.Console.ComputeUiEnabled = true
				config.Console.LegacyPlacements = []string{"0", "1"}
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			service := sat.API.Console.Service
			stripeClient := sat.API.Payments.StripeClient

			up1Proj, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)
			up2Proj, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[1].Projects[0].ID)
			require.NoError(t, err)

			uplink3 := planet.Uplinks[2]
			up3Proj, err := sat.API.DB.Console().Projects().Get(ctx, uplink3.Projects[0].ID)
			require.NoError(t, err)

			up4Proj, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[3].Projects[0].ID)
			require.NoError(t, err)

			require.NotEqual(t, up1Proj.ID, up2Proj.ID)
			require.NotEqual(t, up1Proj.OwnerID, up2Proj.OwnerID)

			userCtx1, err := sat.UserContext(ctx, up1Proj.OwnerID)
			require.NoError(t, err)

			userCtx2, err := sat.UserContext(ctx, up2Proj.OwnerID)
			require.NoError(t, err)

			userCtx3, err := sat.UserContext(ctx, up3Proj.OwnerID)
			require.NoError(t, err)

			getOwnerAndCtx := func(ctx context.Context, proj *console.Project) (user *console.User, userCtx context.Context) {
				user, err := sat.API.DB.Console().Users().Get(ctx, proj.OwnerID)
				require.NoError(t, err)
				userCtx, err = sat.UserContext(ctx, user.ID)
				require.NoError(t, err)
				return
			}

			disableProject := func(ctx context.Context, projectID uuid.UUID) {
				err = sat.API.DB.Console().Projects().UpdateStatus(ctx, projectID, console.ProjectDisabled)
				require.NoError(t, err)
			}

			disabledProject, err := service.CreateProject(userCtx1, console.UpsertProjectInfo{Name: "disabled project"})
			require.NoError(t, err)
			require.NotNil(t, disabledProject)

			disableProject(ctx, disabledProject.ID)

			t.Run("GetUserHasVarPartner", func(t *testing.T) {
				varUser, err := sat.AddUser(ctx, console.CreateUser{
					FullName:  "Var User",
					Email:     "var@storj.test",
					Password:  "password",
					UserAgent: []byte("partner1"),
				}, 1)
				require.NoError(t, err)

				varUserCtx, err := sat.UserContext(ctx, varUser.ID)
				require.NoError(t, err)

				has, err := service.GetUserHasVarPartner(varUserCtx)
				require.NoError(t, err)
				require.True(t, has)

				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Regular User",
					Email:    "reg@storj.test",
					Password: "password",
				}, 1)
				require.NoError(t, err)

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				has, err = service.GetUserHasVarPartner(userCtx)
				require.NoError(t, err)
				require.False(t, has)
			})

			t.Run("GetProject", func(t *testing.T) {
				// Getting own project details should work
				project, err := service.GetProject(userCtx1, up1Proj.ID)
				require.NoError(t, err)
				require.Equal(t, up1Proj.ID, project.ID)

				minProject := service.GetMinimalProject(project)
				require.Equal(t, up1Proj.PublicID, minProject.ID)
				require.Equal(t, up1Proj.Name, minProject.Name)
				require.Equal(t, up1Proj.Description, minProject.Description)
				require.Equal(t, up1Proj.DefaultPlacement, minProject.Placement)
				require.Equal(t, up1Proj.CreatedAt, minProject.CreatedAt)
				require.Equal(t, up1Proj.StorageUsed, minProject.StorageUsed)
				require.Equal(t, up1Proj.BandwidthUsed, minProject.BandwidthUsed)
				require.Equal(t, up1Proj.DefaultVersioning, minProject.Versioning)
				require.Equal(t, up1Proj.MemberCount, minProject.MemberCount)
				require.Equal(t, up1Proj.MemberCount, minProject.MemberCount)
				require.False(t, minProject.HasManagedPassphrase)

				// Getting someone else project details should not work
				project, err = service.GetProject(userCtx1, up2Proj.ID)
				require.Error(t, err)
				require.Nil(t, project)

				_, err = service.GetProject(userCtx1, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))

				passphraseEnc := testrand.Bytes(2 * memory.B)
				managedProject, err := sat.API.DB.Console().Projects().Insert(ctx, &console.Project{
					Name:          "test passphrase enc",
					OwnerID:       up1Proj.OwnerID,
					PassphraseEnc: passphraseEnc,
				})
				require.NoError(t, err)

				minProject = service.GetMinimalProject(managedProject)
				require.True(t, minProject.HasManagedPassphrase)
			})

			t.Run("GetUsersProjects", func(t *testing.T) {
				newProject, err := service.CreateProject(userCtx3, console.UpsertProjectInfo{Name: "new project"})
				require.NoError(t, err)
				require.NotNil(t, newProject)

				disableProject(ctx, newProject.ID)

				projects, err := service.GetUsersProjects(userCtx3)
				require.NoError(t, err)
				require.Len(t, projects, 1)
				require.Equal(t, up3Proj.ID, projects[0].ID)
				require.Zero(t, projects[0].BandwidthUsed)
				require.Zero(t, projects[0].StorageUsed)
				require.Nil(t, projects[0].PassphraseEnc)

				bucket := "testbucket1"
				require.NoError(t, uplink3.TestingCreateBucket(userCtx3, sat, bucket))

				settledAmount := int64(2000)
				now := time.Now().UTC()
				startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

				err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, up3Proj.ID, []byte(bucket), pb.PieceAction_GET, settledAmount, 0, startOfMonth)
				require.NoError(t, err)

				sat.API.Accounting.ProjectUsage.TestSetAsOfSystemInterval(0)

				data := testrand.Bytes(50 * memory.KiB)

				require.NoError(t, uplink3.Upload(ctx, sat, bucket, "1", data))

				segments, err := sat.Metabase.DB.TestingAllSegments(userCtx3)
				require.NoError(t, err)

				service.TestSetNow(func() time.Time {
					return time.Date(now.Year(), now.Month(), 4, 0, 0, 0, 0, time.UTC)
				})

				projects, err = service.GetUsersProjects(userCtx3)
				require.NoError(t, err)
				require.Equal(t, settledAmount, projects[0].BandwidthUsed)
				require.EqualValues(t, segments[0].EncryptedSize, projects[0].StorageUsed)

				// Test IsClassic flag functionality.
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Test User",
					Email:    "getuserprojects@example.com",
					Password: "password",
				}, 2)
				require.NoError(t, err)

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				classicName := "classic project"
				classicProject, err := service.CreateProject(userCtx, console.UpsertProjectInfo{Name: classicName})
				require.NoError(t, err)
				require.NotNil(t, classicProject)

				scope := entitlements.ConvertPublicIDToProjectScope(classicProject.PublicID)
				classicFeatures := entitlements.ProjectFeatures{
					NewBucketPlacements: []storj.PlacementConstraint{0, 1},
				}
				classicFeatBytes, err := json.Marshal(classicFeatures)
				require.NoError(t, err)
				_, err = sat.API.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
					Scope:    scope,
					Features: classicFeatBytes,
				})
				require.NoError(t, err)

				modernName := "modern project"
				modernProject, err := service.CreateProject(userCtx, console.UpsertProjectInfo{Name: modernName})
				require.NoError(t, err)
				require.NotNil(t, modernProject)

				modernScope := entitlements.ConvertPublicIDToProjectScope(modernProject.PublicID)
				modernFeatures := entitlements.ProjectFeatures{
					NewBucketPlacements: []storj.PlacementConstraint{2, 3},
				}
				modernFeatBytes, err := json.Marshal(modernFeatures)
				require.NoError(t, err)
				_, err = sat.API.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
					Scope:    modernScope,
					Features: modernFeatBytes,
				})
				require.NoError(t, err)

				projects, err = service.GetUsersProjects(userCtx)
				require.NoError(t, err)
				require.Len(t, projects, 2)

				var foundClassic, foundModern bool
				for _, proj := range projects {
					switch proj.Name {
					case classicName:
						require.True(t, proj.IsClassic, "classic project should have IsClassic=true")
						foundClassic = true
					case modernName:
						require.False(t, proj.IsClassic, "modern project should have IsClassic=false")
						foundModern = true
					}
				}
				require.True(t, foundClassic, "classic project should be in the list")
				require.True(t, foundModern, "modern project should be in the list")
			})

			t.Run("GetSalt", func(t *testing.T) {
				// Getting project salt as a member should work
				salt, err := service.GetSalt(userCtx1, up1Proj.ID)
				require.NoError(t, err)
				require.NotNil(t, salt)

				// Getting project salt with publicID should work
				salt1, err := service.GetSalt(userCtx1, up1Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, salt1)

				// project.PublicID salt should be the same as project.ID salt
				require.Equal(t, salt, salt1)

				// Getting project salt as a non-member should not work
				salt, err = service.GetSalt(userCtx1, up2Proj.ID)
				require.Error(t, err)
				require.Nil(t, salt)

				salt, err = service.GetSalt(userCtx1, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))
				require.Nil(t, salt)
			})

			t.Run("AddCreditCard fails when payments.CreditCards.Add returns error", func(t *testing.T) {
				// user should be in free tier
				user, userCtx1 := getOwnerAndCtx(ctx, up1Proj)
				require.Equal(t, console.FreeUser, user.Kind)

				// stripecoinpayments.TestPaymentMethodsAttachFailure triggers the underlying mock stripe client to return an error
				// when attaching a payment method to a customer.
				_, err = service.Payments().AddCreditCard(userCtx1, stripe.TestPaymentMethodsAttachFailure)
				require.Error(t, err)

				// user still in free tier
				user, err = service.GetUser(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Equal(t, console.FreeUser, user.Kind)

				cards, err := service.Payments().ListCreditCards(userCtx1)
				require.NoError(t, err)
				require.Len(t, cards, 0)
			})

			t.Run("GetCardSetupSecret", func(t *testing.T) {
				// authenticated user succeeds
				secret, err := service.Payments().GetCardSetupSecret(userCtx1)
				require.NoError(t, err)
				require.NotEmpty(t, secret)

				// unauthenticated context fails
				secret, err = service.Payments().GetCardSetupSecret(ctx)
				require.Error(t, err)
				require.True(t, console.ErrUnauthorized.Has(err))
				require.Empty(t, secret)
			})

			t.Run("AddCreditCard", func(t *testing.T) {
				// user should be in free tier
				user, userCtx1 := getOwnerAndCtx(ctx, up1Proj)
				require.Equal(t, console.FreeUser, user.Kind)

				for i := 0; i < sat.Config.Payments.StripeCoinPayments.MaxCreditCardCount; i++ {
					card, err := service.Payments().AddCreditCard(userCtx1, "test-cc-token"+strconv.Itoa(i))
					require.NoError(t, err)
					require.NotEmpty(t, card)
				}

				// user should be in paid tier
				user, err = service.GetUser(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Equal(t, console.PaidUser, user.Kind)

				cards, err := service.Payments().ListCreditCards(userCtx1)
				require.NoError(t, err)
				require.Len(t, cards, sat.Config.Payments.StripeCoinPayments.MaxCreditCardCount)

				_, err = service.Payments().AddCreditCard(userCtx1, "test-cc-token")
				require.True(t, payments.ErrMaxCreditCards.Has(err))
			})

			t.Run("EnsureUserHasCustomer", func(t *testing.T) {
				// test that a user without associated stripe customer can still
				// add a credit card.
				user, err := sat.API.DB.Console().Users().Insert(ctx, &console.User{
					ID:           testrand.UUID(),
					Email:        "credituser@storj.io",
					PasswordHash: []byte("password"),
				})
				require.NoError(t, err)
				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				customerID, err := sat.API.DB.StripeCoinPayments().Customers().GetCustomerID(userCtx, user.ID)
				require.ErrorIs(t, err, stripe.ErrNoCustomer)
				require.Empty(t, customerID)

				// add a credit card to put the user in the paid tier
				_, err = service.Payments().AddCreditCard(userCtx, "test-cc-token")
				require.NoError(t, err)

				customerID, err = sat.API.DB.StripeCoinPayments().Customers().GetCustomerID(userCtx, user.ID)
				require.NoError(t, err)
				require.NotEmpty(t, customerID)
			})

			t.Run("AddCreditCardByPaymentMethodID", func(t *testing.T) {
				// user should be in free tier
				user, userCtx3 := getOwnerAndCtx(ctx, up3Proj)
				require.Equal(t, console.FreeUser, user.Kind)

				for i := 0; i < sat.Config.Payments.StripeCoinPayments.MaxCreditCardCount; i++ {
					pm, err := stripeClient.PaymentMethods().New(&stripeLib.PaymentMethodParams{
						Type: stripeLib.String(string(stripeLib.PaymentMethodTypeCard)),
						Card: &stripeLib.PaymentMethodCardParams{
							Token: stripeLib.String("test" + strconv.Itoa(i)),
						},
					})
					require.NoError(t, err)

					card, err := service.Payments().AddCardByPaymentMethodID(userCtx3, &payments.AddCardParams{Token: pm.ID}, false)
					require.NoError(t, err)
					require.NotEmpty(t, card)
				}

				cards, err := service.Payments().ListCreditCards(userCtx3)
				require.NoError(t, err)
				require.Len(t, cards, sat.Config.Payments.StripeCoinPayments.MaxCreditCardCount)

				pm, err := stripeClient.PaymentMethods().New(&stripeLib.PaymentMethodParams{
					Type: stripeLib.String(string(stripeLib.PaymentMethodTypeCard)),
					Card: &stripeLib.PaymentMethodCardParams{
						Token: stripeLib.String("test"),
					},
				})
				require.NoError(t, err)

				_, err = service.Payments().AddCardByPaymentMethodID(userCtx3, &payments.AddCardParams{Token: pm.ID}, false)
				require.True(t, payments.ErrMaxCreditCards.Has(err))
			})

			t.Run("Exit trial expiration freeze", func(t *testing.T) {
				freezeService := console.NewAccountFreezeService(sat.DB.Console(), sat.API.Analytics.Service, sat.Config.Console.AccountFreeze)

				user4, userCtx4 := getOwnerAndCtx(ctx, up4Proj)
				require.Equal(t, console.FreeUser, user4.Kind)

				// trial expiration freeze user
				err = freezeService.TrialExpirationFreezeUser(ctx, user4.ID)
				require.NoError(t, err)
				frozen, err := freezeService.IsUserFrozen(ctx, user4.ID, console.TrialExpirationFreeze)
				require.NoError(t, err)
				require.True(t, frozen)

				// add a credit card to put the user in the paid tier
				// and remove the trial expiration freeze event
				_, err = service.Payments().AddCreditCard(userCtx4, "test-cc-token")
				require.NoError(t, err)
				// user should be in paid tier
				user4, err = service.GetUser(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Equal(t, console.PaidUser, user4.Kind)
				limits := sat.Config.Console.UsageLimits
				require.Equal(t, limits.Storage.Paid.Int64(), user4.ProjectStorageLimit)
				require.Equal(t, limits.Bandwidth.Paid.Int64(), user4.ProjectBandwidthLimit)
				require.Equal(t, limits.Segment.Paid, user4.ProjectSegmentLimit)
				require.Equal(t, limits.Project.Paid, user4.ProjectLimit)

				proj, err := sat.API.Console.Service.GetProject(userCtx4, up4Proj.ID)
				require.NoError(t, err)
				require.Equal(t, limits.Storage.Paid, *proj.StorageLimit)
				require.Equal(t, limits.Bandwidth.Paid, *proj.BandwidthLimit)
				require.Equal(t, limits.Segment.Paid, *proj.SegmentLimit)

				// freeze event should be removed
				frozen, err = freezeService.IsUserFrozen(ctx, user4.ID, console.TrialExpirationFreeze)
				require.NoError(t, err)
				require.False(t, frozen)
			})

			t.Run("CreateProject", func(t *testing.T) {
				// Creating a project with a previously used name should fail
				createdProject, err := service.CreateProject(userCtx1, console.UpsertProjectInfo{
					Name: up1Proj.Name,
				})
				require.Error(t, err)
				require.Nil(t, createdProject)
			})

			t.Run("CreateProject when bot account", func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Bot User",
					Email:    "mfauser@mail.test",
				}, 1)
				require.NoError(t, err)

				botStatus := console.PendingBotVerification
				err = sat.API.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
					Status: &botStatus,
				})
				require.NoError(t, err)

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				// Creating a project by bot account must fail.
				createdProject, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
					Name: "test name",
				})
				require.Error(t, err)
				require.True(t, console.ErrBotUser.Has(err))
				require.Nil(t, createdProject)
			})

			t.Run("CreateProject with placement", func(t *testing.T) {
				uid := planet.Uplinks[2].Projects[0].Owner.ID
				err := sat.API.DB.Console().Users().Update(ctx, uid, console.UpdateUserRequest{
					DefaultPlacement: storj.EU,
				})
				require.NoError(t, err)

				user, err := service.GetUser(ctx, uid)
				require.NoError(t, err)

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				p, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
					Name:        "eu-project",
					Description: "project with eu1 default placement",
					CreatedAt:   time.Now(),
				})
				require.NoError(t, err)
				require.Equal(t, console.ProjectActive, *p.Status)
				require.Equal(t, storj.EU, p.DefaultPlacement)
			})

			t.Run("UpdateProject", func(t *testing.T) {
				updatedName := "newName"
				updatedDescription := "newDescription"
				updatedStorageLimit := memory.Size(100)
				updatedBandwidthLimit := memory.Size(100)

				_, userCtx1 := getOwnerAndCtx(ctx, up1Proj)

				// Updating own project should work
				updatedProject, err := service.UpdateProject(userCtx1, up1Proj.ID, console.UpsertProjectInfo{
					Name:           updatedName,
					Description:    updatedDescription,
					StorageLimit:   &updatedStorageLimit,
					BandwidthLimit: &updatedBandwidthLimit,
				})
				require.NoError(t, err)
				require.NotEqual(t, up1Proj.Name, updatedProject.Name)
				require.Equal(t, updatedName, updatedProject.Name)
				require.NotEqual(t, up1Proj.Description, updatedProject.Description)
				require.Equal(t, updatedDescription, updatedProject.Description)
				require.Equal(t, *up1Proj.StorageLimit, *updatedProject.StorageLimit)
				require.Equal(t, *up1Proj.BandwidthLimit, *updatedProject.BandwidthLimit)
				require.Equal(t, updatedStorageLimit, *updatedProject.UserSpecifiedStorageLimit)
				require.Equal(t, updatedBandwidthLimit, *updatedProject.UserSpecifiedBandwidthLimit)
				require.Equal(t, console.ProjectActive, *updatedProject.Status)

				minProject := service.GetMinimalProject(updatedProject)
				require.Equal(t, up1Proj.PublicID, minProject.ID)
				require.Equal(t, updatedName, minProject.Name)
				require.Equal(t, updatedDescription, minProject.Description)

				// Updating someone else project details should not work
				updatedProject, err = service.UpdateProject(userCtx1, up2Proj.ID, console.UpsertProjectInfo{
					Name:           "newName",
					Description:    "TestUpdate",
					StorageLimit:   &updatedStorageLimit,
					BandwidthLimit: &updatedBandwidthLimit,
				})
				require.Error(t, err)
				require.Nil(t, updatedProject)

				// attempting to update a project with bandwidth or storage limits set to 0 should fail
				size0 := new(memory.Size)
				*size0 = 0
				size100 := new(memory.Size)
				*size100 = memory.Size(100)

				up1Proj.StorageLimit = size0
				err = sat.DB.Console().Projects().Update(ctx, up1Proj)
				require.NoError(t, err)

				updateInfo := console.UpsertProjectInfo{
					Name:           "a b c",
					Description:    "1 2 3",
					StorageLimit:   size100,
					BandwidthLimit: size100,
				}
				updatedProject, err = service.UpdateProject(userCtx1, up1Proj.ID, updateInfo)
				require.Error(t, err)
				require.Nil(t, updatedProject)

				up1Proj.StorageLimit = size100
				up1Proj.BandwidthLimit = size0

				err = sat.DB.Console().Projects().Update(ctx, up1Proj)
				require.NoError(t, err)

				updatedProject, err = service.UpdateProject(userCtx1, up1Proj.ID, updateInfo)
				require.Error(t, err)
				require.Nil(t, updatedProject)

				up1Proj.StorageLimit = size100
				up1Proj.BandwidthLimit = size100
				err = sat.DB.Console().Projects().Update(ctx, up1Proj)
				require.NoError(t, err)

				limit := memory.Size(0)
				// should not be able to set limit to zero.
				updatedProject, err = service.UpdateProject(userCtx1, up1Proj.ID, console.UpsertProjectInfo{
					Name:           up1Proj.Name,
					StorageLimit:   &limit,
					BandwidthLimit: &limit,
				})
				require.True(t, console.ErrInvalidProjectLimit.Has(err))
				require.Nil(t, updatedProject)

				// should not be able to set limit more than tier limit.
				biggerStorage := sat.Config.Console.UsageLimits.Storage.Paid + memory.MB
				biggerBandwidth := sat.Config.Console.UsageLimits.Bandwidth.Paid + memory.MB
				updatedProject, err = service.UpdateProject(userCtx1, up1Proj.ID, console.UpsertProjectInfo{
					Name:           up1Proj.Name,
					StorageLimit:   &biggerStorage,
					BandwidthLimit: &biggerBandwidth,
				})
				require.True(t, console.ErrInvalidProjectLimit.Has(err))
				require.Nil(t, updatedProject)

				updatedProject, err = service.UpdateProject(userCtx1, up1Proj.ID, updateInfo)
				require.NoError(t, err)
				require.Equal(t, updateInfo.Name, updatedProject.Name)
				require.Equal(t, updateInfo.Description, updatedProject.Description)
				require.NotNil(t, updatedProject.StorageLimit)
				require.NotNil(t, updatedProject.BandwidthLimit)
				require.Equal(t, updateInfo.StorageLimit, updatedProject.UserSpecifiedStorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, updatedProject.UserSpecifiedBandwidthLimit)

				// updating project with nil limits should skip updating the limits.
				updatedProject, err = service.UpdateProject(userCtx1, up1Proj.ID, console.UpsertProjectInfo{
					Name:           updateInfo.Name,
					StorageLimit:   nil,
					BandwidthLimit: nil,
				})
				require.NoError(t, err)
				require.Equal(t, updateInfo.StorageLimit, updatedProject.UserSpecifiedStorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, updatedProject.UserSpecifiedBandwidthLimit)

				project, err := service.GetProject(userCtx1, up1Proj.ID)
				require.NoError(t, err)
				require.Equal(t, updateInfo.StorageLimit, project.UserSpecifiedStorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, project.UserSpecifiedBandwidthLimit)

				// attempting to update a project with a previously used name should fail
				updatedProject, err = service.UpdateProject(userCtx1, up2Proj.ID, console.UpsertProjectInfo{
					Name: up1Proj.Name,
				})
				require.Error(t, err)
				require.Nil(t, updatedProject)

				user2, userCtx2 := getOwnerAndCtx(ctx, up2Proj)
				_, err = service.AddProjectMembers(userCtx1, up1Proj.ID, []string{user2.Email})
				require.NoError(t, err)
				// Members should not be able to update project.
				_, err = service.UpdateProject(userCtx2, up1Proj.ID, console.UpsertProjectInfo{
					Name: updatedName,
				})
				require.Error(t, err)
				require.True(t, console.ErrUnauthorized.Has(err))
				// remove user2.
				err = service.DeleteProjectMembersAndInvitations(userCtx1, up1Proj.ID, []string{user2.Email})
				require.NoError(t, err)

				_, err = service.UpdateProject(userCtx1, disabledProject.ID, console.UpsertProjectInfo{Name: updatedName})
				require.Error(t, err)
			})

			t.Run("UpdateUserHubspotObjectID", func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Random User",
					Email:    "hubspot@mail.test",
				}, 1)
				require.NoError(t, err)

				expectedObjectID := "hubspot-object-id"

				err = service.UpdateUserHubspotObjectID(ctx, user.ID, expectedObjectID)
				require.NoError(t, err)

				user, err = service.GetUser(ctx, user.ID)
				require.NoError(t, err)
				require.NotNil(t, user.HubspotObjectID)
				require.Equal(t, expectedObjectID, *user.HubspotObjectID)
			})

			t.Run("UpdateUserSpecifiedProjectLimits", func(t *testing.T) {
				updatedStorageLimit := memory.Size(100)
				updatedBandwidthLimit := memory.Size(100)

				_, userCtx1 := getOwnerAndCtx(ctx, up1Proj)

				// Updating own limits should work
				err = service.UpdateUserSpecifiedLimits(userCtx1, up1Proj.ID, console.UpdateLimitsInfo{
					StorageLimit:   &updatedStorageLimit,
					BandwidthLimit: &updatedBandwidthLimit,
				})
				require.NoError(t, err)

				project, err := service.GetProject(userCtx1, up1Proj.ID)
				require.NoError(t, err)
				require.Equal(t, updatedStorageLimit, *project.UserSpecifiedStorageLimit)
				require.Equal(t, updatedBandwidthLimit, *project.UserSpecifiedBandwidthLimit)

				// Updating someone else project limits should not work
				err = service.UpdateUserSpecifiedLimits(userCtx1, up2Proj.ID, console.UpdateLimitsInfo{
					StorageLimit:   &updatedStorageLimit,
					BandwidthLimit: &updatedBandwidthLimit,
				})
				require.Error(t, err)

				limit100 := memory.Size(100)
				// updating only storage limit should work
				err = service.UpdateUserSpecifiedLimits(userCtx1, up1Proj.ID, console.UpdateLimitsInfo{
					StorageLimit: &limit100,
				})
				require.NoError(t, err)

				project, err = service.GetProject(userCtx1, up1Proj.ID)
				require.NoError(t, err)
				require.Equal(t, limit100, *project.UserSpecifiedStorageLimit)
				require.Equal(t, updatedBandwidthLimit, *project.UserSpecifiedBandwidthLimit)

				limit0 := memory.Size(0)
				// passing 0 should remove the limit.
				err = service.UpdateUserSpecifiedLimits(userCtx1, up1Proj.ID, console.UpdateLimitsInfo{
					StorageLimit: &limit0,
				})
				require.NoError(t, err)

				project, err = service.GetProject(userCtx1, up1Proj.ID)
				require.NoError(t, err)
				require.Nil(t, project.UserSpecifiedStorageLimit)
				require.Equal(t, updatedBandwidthLimit, *project.UserSpecifiedBandwidthLimit)

				kind := console.FreeUser
				err = sat.API.DB.Console().Users().Update(ctx, up1Proj.OwnerID, console.UpdateUserRequest{Kind: &kind})
				require.NoError(t, err)
				_, userCtx1 = getOwnerAndCtx(ctx, up1Proj)

				err = service.UpdateUserSpecifiedLimits(userCtx1, up1Proj.ID, console.UpdateLimitsInfo{
					StorageLimit: &limit100,
				})
				// free users should not be able to set limits.
				require.Error(t, err)

				kind = console.NFRUser
				err = sat.API.DB.Console().Users().Update(ctx, up1Proj.OwnerID, console.UpdateUserRequest{Kind: &kind})
				require.NoError(t, err)
				_, userCtx1 = getOwnerAndCtx(ctx, up1Proj)

				err = service.UpdateUserSpecifiedLimits(userCtx1, up1Proj.ID, console.UpdateLimitsInfo{
					StorageLimit: &limit100,
				})
				// nfr users should be able to set limits.
				require.NoError(t, err)

				// reset user back to paid user
				kind = console.PaidUser
				err = sat.API.DB.Console().Users().Update(ctx, up1Proj.OwnerID, console.UpdateUserRequest{Kind: &kind})
				require.NoError(t, err)
				_, userCtx1 = getOwnerAndCtx(ctx, up1Proj)

				err = service.UpdateUserSpecifiedLimits(userCtx1, disabledProject.ID, console.UpdateLimitsInfo{StorageLimit: &limit0})
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("AddProjectMembers", func(t *testing.T) {
				up2User, _ := getOwnerAndCtx(ctx, up2Proj)

				// Adding members to own project should work
				addedUsers, err := service.AddProjectMembers(userCtx1, up1Proj.ID, []string{up2User.Email})
				require.NoError(t, err)
				require.Len(t, addedUsers, 1)
				require.Contains(t, addedUsers, up2User)

				// Adding members to someone else project should not work
				addedUsers, err = service.AddProjectMembers(userCtx1, up2Proj.ID, []string{up2User.Email})
				require.Error(t, err)
				require.Nil(t, addedUsers)

				_, err = service.AddProjectMembers(userCtx1, disabledProject.ID, []string{up2User.Email})
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("GetProjectMembersAndInvitations", func(t *testing.T) {
				// Getting the project members of an own project that one is a part of should work
				userPage, err := service.GetProjectMembersAndInvitations(
					userCtx1,
					up1Proj.ID,
					console.ProjectMembersCursor{Page: 1, Limit: 10},
				)
				require.NoError(t, err)
				require.Len(t, userPage.ProjectMembers, 2)

				// Getting the project members of a foreign project that one is a part of should work
				userPage, err = service.GetProjectMembersAndInvitations(
					userCtx2,
					up1Proj.ID,
					console.ProjectMembersCursor{Page: 1, Limit: 10},
				)
				require.NoError(t, err)
				require.Len(t, userPage.ProjectMembers, 2)

				// Getting the project members of a foreign project that one is not a part of should not work
				userPage, err = service.GetProjectMembersAndInvitations(
					userCtx1,
					up2Proj.ID,
					console.ProjectMembersCursor{Page: 1, Limit: 10},
				)
				require.Error(t, err)
				require.Nil(t, userPage)

				_, err = service.GetProjectMembersAndInvitations(
					userCtx1,
					disabledProject.ID,
					console.ProjectMembersCursor{Page: 1, Limit: 10},
				)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("UpdateProjectMemberRole", func(t *testing.T) {
				newUser, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "New User",
					Email:    "updateRole@example.com",
					Password: "password",
				}, 1)
				require.NoError(t, err)

				newUserCtx, err := sat.UserContext(ctx, newUser.ID)
				require.NoError(t, err)

				_, err = service.AddProjectMembers(userCtx1, up1Proj.ID, []string{newUser.Email})
				require.NoError(t, err)

				// only project owner can change member's role.
				_, err = service.UpdateProjectMemberRole(newUserCtx, up1Proj.OwnerID, up1Proj.ID, console.RoleMember)
				require.True(t, console.ErrForbidden.Has(err))

				// project owner's role can't be changed.
				_, err = service.UpdateProjectMemberRole(userCtx1, up1Proj.OwnerID, up1Proj.ID, console.RoleMember)
				require.True(t, console.ErrConflict.Has(err))

				// project owner can change member's role.
				pm, err := service.UpdateProjectMemberRole(userCtx1, newUser.ID, up1Proj.ID, console.RoleAdmin)
				require.NoError(t, err)
				require.EqualValues(t, console.RoleAdmin, pm.Role)

				_, err = service.UpdateProjectMemberRole(userCtx1, newUser.ID, disabledProject.ID, console.RoleAdmin)
				require.Error(t, err)
			})

			t.Run("DeleteProjectMembersAndInvitations", func(t *testing.T) {
				user1, user1Ctx := getOwnerAndCtx(ctx, up1Proj)
				_, user2Ctx := getOwnerAndCtx(ctx, up2Proj)

				invitedUser, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Test User",
					Email:    "test@mail.test",
				}, 1)
				require.NoError(t, err)

				invitedUserCtx, err := sat.UserContext(ctx, invitedUser.ID)
				require.NoError(t, err)

				for _, id := range []uuid.UUID{up1Proj.ID, up2Proj.ID} {
					_, err = sat.DB.Console().ProjectInvitations().Upsert(ctx, &console.ProjectInvitation{
						ProjectID: id,
						Email:     invitedUser.Email,
					})
					require.NoError(t, err)
				}

				// You should not be able to remove someone from a project that you aren't a member of.
				err = service.DeleteProjectMembersAndInvitations(user1Ctx, up2Proj.ID, []string{invitedUser.Email})
				require.Error(t, err)

				// Project owners should not be able to be removed.
				err = service.DeleteProjectMembersAndInvitations(user2Ctx, up1Proj.ID, []string{user1.Email})
				require.Error(t, err)

				// An invalid email should cause the operation to fail.
				err = service.DeleteProjectMembersAndInvitations(
					user2Ctx,
					up2Proj.ID,
					[]string{invitedUser.Email, "nobody@mail.test"},
				)
				require.Error(t, err)

				_, err = sat.DB.Console().ProjectInvitations().Get(ctx, up2Proj.ID, invitedUser.Email)
				require.NoError(t, err)

				// Members and invitations should be removed.
				err = service.DeleteProjectMembersAndInvitations(user2Ctx, up2Proj.ID, []string{invitedUser.Email, user1.Email})
				require.NoError(t, err)

				_, err = sat.DB.Console().ProjectInvitations().Get(ctx, up2Proj.ID, invitedUser.Email)
				require.ErrorIs(t, err, sql.ErrNoRows)

				memberships, err := sat.DB.Console().ProjectMembers().GetByMemberID(ctx, user1.ID)
				require.NoError(t, err)
				require.Len(t, memberships, 2)
				require.NotEqual(t, up2Proj.ID, memberships[0].ProjectID)

				err = service.RespondToProjectInvitation(invitedUserCtx, up1Proj.ID, console.ProjectInvitationAccept)
				require.NoError(t, err)

				invitedMember, err := sat.DB.Console().ProjectMembers().GetByMemberIDAndProjectID(ctx, invitedUser.ID, up1Proj.ID)
				require.NoError(t, err)
				require.Equal(t, console.RoleMember, invitedMember.Role)

				// Members with console.RoleMember status can't delete other members.
				err = service.DeleteProjectMembersAndInvitations(invitedUserCtx, up1Proj.ID, []string{invitedUser.Email, user1.Email})
				require.True(t, console.ErrForbidden.Has(err))

				// Members with console.RoleMember status can delete themselves.
				err = service.DeleteProjectMembersAndInvitations(invitedUserCtx, up1Proj.ID, []string{invitedUser.Email})
				require.NoError(t, err)

				_, err = sat.DB.Console().ProjectMembers().GetByMemberIDAndProjectID(ctx, invitedMember.MemberID, up1Proj.ID)
				require.ErrorIs(t, err, sql.ErrNoRows)

				err = service.DeleteProjectMembersAndInvitations(
					userCtx1,
					disabledProject.ID,
					[]string{invitedUser.Email, "nobody@mail.test"},
				)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("CreateAPIKey", func(t *testing.T) {
				createdAPIKey, _, err := service.CreateAPIKey(userCtx2, up2Proj.ID, "test key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)
				require.NotNil(t, createdAPIKey)
				require.Equal(t, up2Proj.OwnerID, createdAPIKey.CreatedBy)
				require.False(t, createdAPIKey.Version.SupportsObjectLock())
				require.False(t, createdAPIKey.Version.SupportsAuditability())

				_, _, err = service.CreateAPIKey(userCtx1, disabledProject.ID, "test key", macaroon.APIKeyVersionMin)
				require.True(t, console.ErrUnauthorized.Has(err))

				createdAPIKey, _, err = service.CreateAPIKey(userCtx2, up2Proj.ID, "test key 1", macaroon.APIKeyVersionObjectLock)
				require.NoError(t, err)
				require.NotNil(t, createdAPIKey)
				require.Equal(t, macaroon.APIKeyVersionObjectLock, createdAPIKey.Version)
				require.True(t, createdAPIKey.Version.SupportsObjectLock())
				require.False(t, createdAPIKey.Version.SupportsAuditability())

				createdAPIKey, _, err = service.CreateAPIKey(userCtx2, up2Proj.ID, "test key 2", macaroon.APIKeyVersionAuditable)
				require.NoError(t, err)
				require.NotNil(t, createdAPIKey)
				require.Equal(t, macaroon.APIKeyVersionAuditable, createdAPIKey.Version)
				require.False(t, createdAPIKey.Version.SupportsObjectLock())
				require.True(t, createdAPIKey.Version.SupportsAuditability())

				createdAPIKey, _, err = service.CreateAPIKey(userCtx2, up2Proj.ID, "test key 3", macaroon.APIKeyVersionAuditable|macaroon.APIKeyVersionObjectLock)
				require.NoError(t, err)
				require.NotNil(t, createdAPIKey)
				require.Equal(t, macaroon.APIKeyVersionAuditable|macaroon.APIKeyVersionObjectLock, createdAPIKey.Version)
				require.True(t, createdAPIKey.Version.SupportsObjectLock())
				require.True(t, createdAPIKey.Version.SupportsAuditability())
			})

			t.Run("CreateDomain", func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Domain Creator",
					Email:    "create_domain@example.test",
				}, 1)
				require.NoError(t, err)

				project, err := sat.AddProject(ctx, user.ID, "Create Domain")
				require.NoError(t, err)

				domain := console.Domain{
					ProjectPublicID: project.PublicID,
					CreatedBy:       user.ID,
					Subdomain:       "test.example.test",
					Prefix:          "test",
					AccessID:        "test",
				}

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				createdDomain, err := service.CreateDomain(userCtx, domain)
				require.True(t, console.ErrNotPaidTier.Has(err))
				require.Nil(t, createdDomain)

				paidKind := console.PaidUser
				err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind})
				require.NoError(t, err)

				userCtx, err = sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				createdDomain, err = service.CreateDomain(userCtx, domain)
				require.NoError(t, err)
				require.NotNil(t, createdDomain)
				require.Equal(t, user.ID, createdDomain.CreatedBy)

				// Creating a domain with the same subdomain should fail.
				createdDomain, err = service.CreateDomain(userCtx, domain)
				require.True(t, console.ErrSubdomainAlreadyExists.Has(err))
				require.Nil(t, createdDomain)

				domain.ProjectPublicID = disabledProject.PublicID
				domain.CreatedBy = disabledProject.OwnerID

				_, err = service.CreateDomain(userCtx1, domain)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("DeleteAPIKeys", func(t *testing.T) {
				owner, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Owner Name",
					Email:    "deletekeys_owner@example.com",
				}, 1)
				require.NoError(t, err)
				member, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Member Name",
					Email:    "deletekeys_member@example.com",
				}, 1)
				require.NoError(t, err)

				pr, err := sat.AddProject(ctx, owner.ID, "Delete Keys Project")
				require.NoError(t, err)
				require.NotNil(t, pr)

				ownerCtx, err := sat.UserContext(ctx, owner.ID)
				require.NoError(t, err)
				memberCtx, err := sat.UserContext(ctx, member.ID)
				require.NoError(t, err)

				_, err = service.AddProjectMembers(ownerCtx, pr.ID, []string{member.Email})
				require.NoError(t, err)

				_, err = service.UpdateProjectMemberRole(ownerCtx, member.ID, pr.ID, console.RoleMember)
				require.NoError(t, err)

				ownerKey, _, err := service.CreateAPIKey(ownerCtx, pr.ID, "owner's key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)
				require.NotNil(t, ownerKey)
				memberKey, _, err := service.CreateAPIKey(memberCtx, pr.ID, "member's key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)
				require.NotNil(t, memberKey)

				// member can't delete owner's key.
				err = service.DeleteAPIKeys(memberCtx, []uuid.UUID{ownerKey.ID, memberKey.ID})
				require.True(t, console.ErrForbidden.Has(err))

				// owner can delete all the keys.
				err = service.DeleteAPIKeys(ownerCtx, []uuid.UUID{ownerKey.ID, memberKey.ID})
				require.NoError(t, err)

				ownerKey, _, err = service.CreateAPIKey(ownerCtx, pr.ID, "owner's key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)
				require.NotNil(t, ownerKey)
				memberKey, _, err = service.CreateAPIKey(memberCtx, pr.ID, "member's key", macaroon.APIKeyVersionMin)
				require.NoError(t, err)
				require.NotNil(t, memberKey)

				_, err = service.UpdateProjectMemberRole(ownerCtx, member.ID, pr.ID, console.RoleAdmin)
				require.NoError(t, err)

				// admin can delete all the keys.
				err = service.DeleteAPIKeys(memberCtx, []uuid.UUID{ownerKey.ID, memberKey.ID})
				require.NoError(t, err)
			})

			t.Run("GetProjectMember", func(t *testing.T) {
				owner, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Owner Name",
					Email:    "get_member_owner@example.com",
				}, 1)
				require.NoError(t, err)
				member, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Member Name",
					Email:    "get_member_member@example.com",
				}, 1)
				require.NoError(t, err)

				pr, err := sat.AddProject(ctx, owner.ID, "Get Project Member")
				require.NoError(t, err)
				require.NotNil(t, pr)

				ownerCtx, err := sat.UserContext(ctx, owner.ID)
				require.NoError(t, err)
				memberCtx, err := sat.UserContext(ctx, member.ID)
				require.NoError(t, err)

				pm, err := service.GetProjectMember(memberCtx, owner.ID, pr.ID)
				require.True(t, console.ErrNoMembership.Has(err))
				require.Nil(t, pm)

				_, err = service.AddProjectMembers(ownerCtx, pr.ID, []string{member.Email})
				require.NoError(t, err)

				pm, err = service.GetProjectMember(memberCtx, member.ID, pr.ID)
				require.NoError(t, err)
				require.Equal(t, console.RoleMember, pm.Role)

				_, err = service.GetProjectMember(userCtx1, member.ID, disabledProject.ID)
				require.True(t, console.ErrNoMembership.Has(err))
			})

			t.Run("GetProjectUsageLimits", func(t *testing.T) {
				require.NoError(t, planet.Uplinks[1].TestingCreateBucket(ctx, sat, "testbucket"))

				bandwidthLimit := sat.Config.Console.UsageLimits.Bandwidth.Free
				storageLimit := sat.Config.Console.UsageLimits.Storage.Free
				bucketsLimit := int64(sat.Config.Metainfo.ProjectLimits.MaxBuckets)

				limits1, err := service.GetProjectUsageLimits(userCtx2, up2Proj.ID)
				require.NoError(t, err)
				require.NotNil(t, limits1)

				// Get usage limits with publicID
				limits2, err := service.GetProjectUsageLimits(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)

				// limits gotten by ID and publicID should be the same
				require.Equal(t, storageLimit.Int64(), limits1.StorageLimit)
				require.Nil(t, limits1.UserSetStorageLimit)
				require.Equal(t, bandwidthLimit.Int64(), limits1.BandwidthLimit)
				require.Nil(t, limits1.UserSetBandwidthLimit)
				require.Equal(t, int64(1), limits1.BucketsUsed)
				require.Equal(t, bucketsLimit, limits1.BucketsLimit)
				require.Equal(t, storageLimit.Int64(), limits2.StorageLimit)
				require.Nil(t, limits2.UserSetStorageLimit)
				require.Equal(t, bandwidthLimit.Int64(), limits2.BandwidthLimit)
				require.Nil(t, limits2.UserSetBandwidthLimit)
				require.Equal(t, int64(1), limits2.BucketsUsed)
				require.Equal(t, bucketsLimit, limits2.BucketsLimit)

				// update project's limits
				updatedStorageLimit := memory.Size(100) + memory.TB
				userSpecifiedStorage := updatedStorageLimit / 2
				updatedBandwidthLimit := memory.Size(100) + memory.TB
				userSpecifiedBandwidth := updatedBandwidthLimit / 2
				up2Proj.StorageLimit = &updatedStorageLimit
				up2Proj.UserSpecifiedStorageLimit = &userSpecifiedStorage
				up2Proj.BandwidthLimit = &updatedBandwidthLimit
				up2Proj.UserSpecifiedBandwidthLimit = &userSpecifiedBandwidth
				err = sat.DB.Console().Projects().Update(ctx, up2Proj)
				require.NoError(t, err)

				updatedBucketsLimit := 20
				err = sat.DB.Console().Projects().UpdateBucketLimit(ctx, up2Proj.ID, &updatedBucketsLimit)
				require.NoError(t, err)

				limits1, err = service.GetProjectUsageLimits(userCtx2, up2Proj.ID)
				require.NoError(t, err)
				require.NotNil(t, limits1)

				// Get usage limits with publicID
				limits2, err = service.GetProjectUsageLimits(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)

				// limits gotten by ID and publicID should be the same
				require.Equal(t, updatedStorageLimit.Int64(), limits1.StorageLimit)
				require.Equal(t, userSpecifiedStorage.Int64(), *limits1.UserSetStorageLimit)
				require.Equal(t, updatedBandwidthLimit.Int64(), limits1.BandwidthLimit)
				require.Equal(t, userSpecifiedBandwidth.Int64(), *limits1.UserSetBandwidthLimit)
				require.Equal(t, int64(updatedBucketsLimit), limits1.BucketsLimit)
				require.Equal(t, updatedStorageLimit.Int64(), limits2.StorageLimit)
				require.Equal(t, userSpecifiedStorage.Int64(), *limits2.UserSetStorageLimit)
				require.Equal(t, updatedBandwidthLimit.Int64(), limits2.BandwidthLimit)
				require.Equal(t, userSpecifiedBandwidth.Int64(), *limits2.UserSetBandwidthLimit)
				require.Equal(t, int64(updatedBucketsLimit), limits2.BucketsLimit)

				bucket := "testbucket1"
				err = planet.Uplinks[1].TestingCreateBucket(ctx, sat, bucket)
				require.NoError(t, err)

				now := time.Now().UTC()
				allocatedAmount := int64(1000)
				settledAmount := int64(2000)
				startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
				thirdDayOfMonth := time.Date(now.Year(), now.Month(), 3, 0, 0, 0, 0, time.UTC)

				// set now as third day of the month.
				service.TestSetNow(func() time.Time {
					return thirdDayOfMonth
				})

				// add allocated and settled bandwidth for the beginning of the month.
				err = sat.DB.Orders().
					UpdateBucketBandwidthAllocation(ctx, up2Proj.ID, []byte(bucket), pb.PieceAction_GET, allocatedAmount, startOfMonth)
				require.NoError(t, err)
				err = sat.DB.Orders().
					UpdateBucketBandwidthSettle(ctx, up2Proj.ID, []byte(bucket), pb.PieceAction_GET, settledAmount, 0, startOfMonth)
				require.NoError(t, err)

				sat.API.Accounting.ProjectUsage.TestSetAsOfSystemInterval(0)

				// at this point only allocated traffic is expected.
				limits2, err = service.GetProjectUsageLimits(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)
				require.Equal(t, int64(2), limits2.BucketsUsed)
				require.Equal(t, allocatedAmount, limits2.BandwidthUsed)

				// set now as fourth day of the month.
				service.TestSetNow(func() time.Time {
					return time.Date(now.Year(), now.Month(), 4, 0, 0, 0, 0, time.UTC)
				})

				// at this point only settled traffic for the first day is expected.
				limits2, err = service.GetProjectUsageLimits(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)
				require.Equal(t, settledAmount, limits2.BandwidthUsed)

				// add settled traffic for the third day of the month.
				err = sat.DB.Orders().
					UpdateBucketBandwidthSettle(ctx, up2Proj.ID, []byte(bucket), pb.PieceAction_GET, settledAmount, 0, thirdDayOfMonth)
				require.NoError(t, err)

				// at this point only settled traffic for the first day is expected because now is still set to fourth day.
				limits2, err = service.GetProjectUsageLimits(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)
				require.Equal(t, settledAmount, limits2.BandwidthUsed)

				// set now as sixth day of the month.
				service.TestSetNow(func() time.Time {
					return time.Date(now.Year(), now.Month(), 6, 0, 0, 0, 0, time.UTC)
				})

				// at this point only settled traffic for the first and third days is expected.
				limits2, err = service.GetProjectUsageLimits(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)
				require.Equal(t, settledAmount+settledAmount, limits2.BandwidthUsed)

				_, err = service.GetProjectUsageLimits(userCtx1, disabledProject.PublicID)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("GetAllBucketNames", func(t *testing.T) {
				bucket1 := buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      "testBucket1",
					ProjectID: up2Proj.ID,
				}

				bucket2 := buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      "testBucket2",
					ProjectID: up2Proj.ID,
				}

				_, err := sat.API.Buckets.Service.CreateBucket(userCtx2, bucket1)
				require.NoError(t, err)

				_, err = sat.API.Buckets.Service.CreateBucket(userCtx2, bucket2)
				require.NoError(t, err)

				bucketNames, err := service.GetAllBucketNames(userCtx2, up2Proj.ID)
				require.NoError(t, err)
				require.Equal(t, bucket1.Name, bucketNames[0])
				require.Equal(t, bucket2.Name, bucketNames[1])

				bucketNames, err = service.GetAllBucketNames(userCtx2, up2Proj.PublicID)
				require.NoError(t, err)
				require.Equal(t, bucket1.Name, bucketNames[0])
				require.Equal(t, bucket2.Name, bucketNames[1])

				// Getting someone else buckets should not work
				bucketsForUnauthorizedUser, err := service.GetAllBucketNames(userCtx1, up2Proj.ID)
				require.Error(t, err)
				require.Nil(t, bucketsForUnauthorizedUser)

				_, err = service.GetAllBucketNames(userCtx1, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("GetBucketTotals", func(t *testing.T) {
				list, err := sat.DB.Buckets().ListBuckets(ctx, up2Proj.ID, buckets.ListOptions{Direction: buckets.DirectionForward}, macaroon.AllowedBuckets{All: true})
				require.NoError(t, err)
				for i, item := range list.Items {
					item.Placement = storj.PlacementConstraint(i)
					if i > len(placements)-1 {
						item.Placement = storj.PlacementConstraint(len(placements) - 1)
					}
					b, err := sat.DB.Buckets().UpdateBucket(ctx, item)
					require.NoError(t, err)
					require.Equal(t, i, int(b.Placement))
				}

				before := time.Now()
				since := before.Add(-time.Hour * 24)

				bt, err := service.GetBucketTotals(userCtx2, up2Proj.ID, accounting.BucketUsageCursor{Limit: 100, Page: 1}, since, before)
				require.NoError(t, err)
				for _, b := range bt.BucketUsages {
					require.Equal(t, placements[int(b.DefaultPlacement)], b.Location)
				}

				_, err = service.GetBucketTotals(userCtx1, disabledProject.ID, accounting.BucketUsageCursor{Limit: 100, Page: 1}, since, before)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("GetSingleBucketTotals", func(t *testing.T) {
				bucketName := "test-single-bucket"

				err = planet.Uplinks[1].CreateBucket(ctx, sat, bucketName)
				require.NoError(t, err)

				storedBucket, err := sat.DB.Buckets().GetBucket(ctx, []byte(bucketName), up2Proj.ID)
				require.NoError(t, err)

				_, err = service.GetSingleBucketTotals(userCtx1, up2Proj.ID, storedBucket.Name, time.Now())
				require.True(t, console.ErrUnauthorized.Has(err))

				client, err := planet.Uplinks[1].Projects[0].DialMetainfo(ctx)
				require.NoError(t, err)
				defer func() {
					require.NoError(t, client.Close())
				}()

				err = client.SetBucketVersioning(ctx, metaclient.SetBucketVersioningParams{
					Name:       []byte(storedBucket.Name),
					Versioning: true,
				})
				require.NoError(t, err)

				storedBucket.Placement = storj.EU

				_, err = sat.DB.Buckets().UpdateBucket(ctx, storedBucket)
				require.NoError(t, err)

				bt, err := service.GetSingleBucketTotals(userCtx2, up2Proj.ID, storedBucket.Name, time.Now())
				require.NoError(t, err)
				require.Equal(t, storj.EU, bt.DefaultPlacement)
				require.Equal(t, buckets.VersioningEnabled, bt.Versioning)

				_, err = service.GetSingleBucketTotals(userCtx1, disabledProject.ID, storedBucket.Name, time.Now())
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("GetBucketMetadata", func(t *testing.T) {
				list, err := sat.DB.Buckets().ListBuckets(ctx, up2Proj.ID, buckets.ListOptions{Direction: buckets.DirectionForward}, macaroon.AllowedBuckets{All: true})
				require.NoError(t, err)
				bp, err := service.GetBucketMetadata(userCtx2, up2Proj.ID)
				require.NoError(t, err)
				for _, b := range bp {
					var found bool
					for _, item := range list.Items {
						if item.Name == b.Name {
							found = true
							require.Equal(t, item.Placement, b.Placement.DefaultPlacement)
							require.Equal(t, placements[int(item.Placement)], b.Placement.Location)
							require.Equal(t, item.Versioning, b.Versioning)
							break
						}
					}
					if found {
						continue
					}
					require.Fail(t, "bucket name not in list", b.Name)
				}

				_, err = service.GetBucketMetadata(userCtx1, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))
			})
			t.Run("DeleteDomain", func(t *testing.T) {
				user, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Domain Deleter",
					Email:    "delete_domain@example.test",
				}, 1)
				require.NoError(t, err)

				paidKind := console.PaidUser
				err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &paidKind})
				require.NoError(t, err)

				project, err := sat.AddProject(ctx, user.ID, "Delete Domain")
				require.NoError(t, err)

				d := console.Domain{
					ProjectID:       project.ID,
					ProjectPublicID: project.PublicID,
					CreatedBy:       project.OwnerID,
					Subdomain:       "delete.example.com",
					Prefix:          "delete",
					AccessID:        "delete",
				}

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				createdDomain, err := service.CreateDomain(userCtx, d)
				require.NoError(t, err)
				require.NotNil(t, createdDomain)

				err = service.DeleteDomain(userCtx, project.PublicID, createdDomain.Subdomain)
				require.NoError(t, err)

				createdDomain, err = service.CreateDomain(userCtx, d)
				require.NoError(t, err)
				require.NotNil(t, createdDomain)

				err = service.DeleteDomain(userCtx2, project.PublicID, createdDomain.Subdomain)
				require.True(t, console.ErrUnauthorized.Has(err))

				anotherUser, err := sat.API.DB.Console().Users().Get(ctx, up2Proj.OwnerID)
				require.NoError(t, err)

				_, err = service.AddProjectMembers(userCtx, project.PublicID, []string{anotherUser.Email})
				require.NoError(t, err)

				_, err = service.UpdateProjectMemberRole(userCtx, anotherUser.ID, project.PublicID, console.RoleMember)
				require.NoError(t, err)

				err = service.DeleteDomain(userCtx2, project.PublicID, createdDomain.Subdomain)
				require.True(t, console.ErrForbidden.Has(err))

				_, err = service.UpdateProjectMemberRole(userCtx, anotherUser.ID, project.PublicID, console.RoleAdmin)
				require.NoError(t, err)

				err = service.DeleteDomain(userCtx2, project.PublicID, createdDomain.Subdomain)
				require.NoError(t, err)
			})

			t.Run("DeleteAPIKeyByNameAndProjectID", func(t *testing.T) {
				secret, err := macaroon.NewSecret()
				require.NoError(t, err)

				key, err := macaroon.NewAPIKey(secret)
				require.NoError(t, err)

				apikey := console.APIKeyInfo{
					Name:      "test",
					ProjectID: up2Proj.ID,
					Secret:    secret,
				}

				createdKey, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				info, err := sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.NoError(t, err)
				require.NotNil(t, info)

				// Deleting someone else api keys should not work
				err = service.DeleteAPIKeyByNameAndProjectID(userCtx1, apikey.Name, up2Proj.ID)
				require.Error(t, err)

				err = service.DeleteAPIKeyByNameAndProjectID(userCtx2, apikey.Name, up2Proj.ID)
				require.NoError(t, err)

				info, err = sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.Error(t, err)
				require.Nil(t, info)

				// test deleting by project.publicID
				createdKey, err = sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				info, err = sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.NoError(t, err)
				require.NotNil(t, info)

				// deleting by project.publicID
				err = service.DeleteAPIKeyByNameAndProjectID(userCtx2, apikey.Name, up2Proj.PublicID)
				require.NoError(t, err)

				info, err = sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.Error(t, err)
				require.Nil(t, info)

				err = service.DeleteAPIKeyByNameAndProjectID(userCtx1, apikey.Name, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))
			})
			t.Run("ApplyFreeTierCoupon", func(t *testing.T) {
				// testplanet applies the free tier coupon first, so we need to change it in order
				// to verify that ApplyFreeTierCoupon really works.
				freeTier := sat.Config.Payments.StripeCoinPayments.StripeFreeTierCouponID
				coupon3, err := service.Payments().ApplyCoupon(userCtx1, stripe.MockCouponID3)
				require.NoError(t, err)
				require.NotNil(t, coupon3)
				require.NotEqual(t, freeTier, coupon3.ID)

				coupon, err := service.Payments().ApplyFreeTierCoupon(userCtx1)
				require.NoError(t, err)
				require.NotNil(t, coupon)
				require.Equal(t, freeTier, coupon.ID)

				coupon, err = sat.API.Payments.Accounts.Coupons().GetByUserID(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Equal(t, freeTier, coupon.ID)
			})
			t.Run("ApplyFreeTierCoupon fails with unknown user", func(t *testing.T) {
				coupon, err := service.Payments().ApplyFreeTierCoupon(ctx)
				require.Error(t, err)
				require.Nil(t, coupon)
			})
			t.Run("ApplyCoupon", func(t *testing.T) {
				id := stripe.MockCouponID2
				coupon, err := service.Payments().ApplyCoupon(userCtx2, id)
				require.NoError(t, err)
				require.NotNil(t, coupon)
				require.Equal(t, id, coupon.ID)

				coupon, err = sat.API.Payments.Accounts.Coupons().GetByUserID(ctx, up2Proj.OwnerID)
				require.NoError(t, err)
				require.Equal(t, id, coupon.ID)
			})
			t.Run("ApplyCoupon fails with unknown user", func(t *testing.T) {
				id := stripe.MockCouponID2
				coupon, err := service.Payments().ApplyCoupon(ctx, id)
				require.Error(t, err)
				require.Nil(t, coupon)
			})
			t.Run("ApplyCoupon fails with unknown coupon ID", func(t *testing.T) {
				coupon, err := service.Payments().ApplyCoupon(userCtx2, "unknown_coupon_id")
				require.Error(t, err)
				require.Nil(t, coupon)
			})
			t.Run("UpdatePackage", func(t *testing.T) {
				packagePlan := "package-plan-1"
				purchaseTime := time.Now()

				check := func() {
					dbPackagePlan, dbPurchaseTime, err := sat.DB.StripeCoinPayments().
						Customers().
						GetPackageInfo(ctx, up1Proj.OwnerID)
					require.NoError(t, err)
					require.NotNil(t, dbPackagePlan)
					require.NotNil(t, dbPurchaseTime)
					require.Equal(t, packagePlan, *dbPackagePlan)
					require.Equal(t, dbPurchaseTime.Truncate(time.Millisecond), dbPurchaseTime.Truncate(time.Millisecond))
				}

				require.NoError(t, service.Payments().UpdatePackage(userCtx1, packagePlan, purchaseTime))
				check()

				// Check values can't be overwritten
				err = service.Payments().UpdatePackage(userCtx1, "different-package-plan", time.Now())
				require.Error(t, err)
				require.True(t, console.ErrAlreadyHasPackage.Has(err))

				check()
			})
			t.Run("ApplyCredit fails when payments.Balances.ApplyCredit returns an error", func(t *testing.T) {
				require.Error(t, service.Payments().ApplyCredit(userCtx1, 1000, stripe.MockCBTXsNewFailure))
				btxs, err := sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Zero(t, len(btxs))
			})
			t.Run("ApplyCredit", func(t *testing.T) {
				amount := int64(1000)
				desc := "test"
				require.NoError(t, service.Payments().ApplyCredit(userCtx1, 1000, desc))
				btxs, err := sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Len(t, btxs, 1)
				require.Equal(t, amount, btxs[0].Amount)
				require.Equal(t, desc, btxs[0].Description)

				// test same description results in no new credit
				require.NoError(t, service.Payments().ApplyCredit(userCtx1, 1000, desc))
				btxs, err = sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Len(t, btxs, 1)

				// test different description results in new credit
				require.NoError(t, service.Payments().ApplyCredit(userCtx1, 1000, "new desc"))
				btxs, err = sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Proj.OwnerID)
				require.NoError(t, err)
				require.Len(t, btxs, 2)
			})
			t.Run("ApplyCredit fails with unknown user", func(t *testing.T) {
				require.Error(t, service.Payments().ApplyCredit(ctx, 1000, "test"))
			})
			t.Run("GetEmissionImpact", func(t *testing.T) {
				pr, err := sat.AddProject(userCtx1, up1Proj.OwnerID, "emission test")
				require.NoError(t, err)
				require.NotNil(t, pr)

				// Getting project emission impact as a member should work
				impact, err := service.GetEmissionImpact(userCtx1, pr.ID)
				require.NoError(t, err)
				require.NotNil(t, impact)
				require.EqualValues(t, console.EmissionImpactResponse{}, *impact)

				// Getting project salt as a non-member should not work
				impact, err = service.GetEmissionImpact(userCtx2, pr.ID)
				require.Error(t, err)
				require.Nil(t, impact)

				err = sat.API.Accounting.ProjectUsage.UpdateProjectStorageAndSegmentUsage(userCtx1, accounting.ProjectLimits{ProjectID: pr.ID}, (2 * memory.TB).Int64(), 0)
				require.NoError(t, err)

				now := time.Now().UTC()
				service.TestSetNow(func() time.Time {
					return now.Add(365.25 * 24 * time.Hour)
				})

				zeroValue := float64(0)

				impact, err = service.GetEmissionImpact(userCtx1, pr.ID)
				require.NoError(t, err)
				require.NotNil(t, impact)
				require.Greater(t, impact.StorjImpact, zeroValue)
				require.Greater(t, impact.HyperscalerImpact, zeroValue)
				require.Greater(t, impact.SavedTrees, int64(0))

				_, err = service.GetEmissionImpact(userCtx1, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))
			})
			t.Run("GetUsageReport", func(t *testing.T) {
				usr, err := sat.AddUser(ctx, console.CreateUser{
					FullName: "Test Usage Report",
					Email:    "test.report@mail.test",
				}, 2)
				require.NoError(t, err)

				usrCtx, err := sat.UserContext(ctx, usr.ID)
				require.NoError(t, err)

				createApiKey := func(project *console.Project) *macaroon.APIKey {
					secret, err := macaroon.NewSecret()
					require.NoError(t, err)

					key, err := macaroon.NewAPIKey(secret)
					require.NoError(t, err)

					keyInfo := console.APIKeyInfo{
						Name:      project.Name,
						ProjectID: project.ID,
						Secret:    secret,
					}
					_, err = sat.DB.Console().APIKeys().Create(ctx, key.Head(), keyInfo)
					require.NoError(t, err)

					return key
				}

				pr1, err := sat.AddProject(ctx, usr.ID, "report test 1")
				require.NoError(t, err)
				createdAPIKey := createApiKey(pr1)

				pr2, err := sat.AddProject(ctx, usr.ID, "report test 2")
				require.NoError(t, err)
				createdAPIKey2 := createApiKey(pr2)

				endpoint := sat.API.Metainfo.Endpoint

				bucket1 := "bucket1"
				_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
					Header: &pb.RequestHeader{ApiKey: createdAPIKey.SerializeRaw()},
					Name:   []byte(bucket1),
				})
				require.NoError(t, err)

				bucket2 := "testbuckettwo"
				_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
					Header: &pb.RequestHeader{ApiKey: createdAPIKey2.SerializeRaw()},
					Name:   []byte(bucket2),
				})
				require.NoError(t, err)

				now := time.Now()
				inHalfAnHour := now.Add(30 * time.Minute)
				inAnHour := now.Add(time.Hour)

				params := console.GetUsageReportParam{
					Since:     now,
					Before:    inAnHour,
					ProjectID: pr1.PublicID,
				}

				items, err := service.GetUsageReport(userCtx2, params)
				require.True(t, console.ErrUnauthorized.Has(err))
				require.Nil(t, items)

				amount := memory.Size(1000)
				err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, pr1.ID, []byte(bucket1), pb.PieceAction_GET, amount.Int64(), 0, inHalfAnHour)
				require.NoError(t, err)
				err = sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, pr2.ID, []byte(bucket2), pb.PieceAction_GET, amount.Int64(), 0, inHalfAnHour)
				require.NoError(t, err)

				items, err = service.GetUsageReport(usrCtx, params)
				require.NoError(t, err)
				require.Len(t, items, 1)
				require.Equal(t, pr1.PublicID, items[0].ProjectPublicID)
				require.Equal(t, bucket1, items[0].BucketName)
				require.Equal(t, amount.GB(), items[0].Egress)

				params.ProjectID = pr2.PublicID
				items, err = service.GetUsageReport(usrCtx, params)
				require.NoError(t, err)
				require.Len(t, items, 1)
				require.Equal(t, pr2.PublicID, items[0].ProjectPublicID)
				require.Equal(t, bucket2, items[0].BucketName)
				require.Equal(t, amount.GB(), items[0].Egress)

				params.ProjectID = uuid.UUID{}
				items, err = service.GetUsageReport(usrCtx, params)
				require.NoError(t, err)
				require.Len(t, items, 2)

				params.ProjectID = disabledProject.ID
				_, err = service.GetUsageReport(userCtx1, params)
				require.True(t, console.ErrUnauthorized.Has(err))
			})

			t.Run("GetProjectConfig", func(t *testing.T) {
				newLimit := 5
				err = sat.DB.Console().Users().Update(ctx, up1Proj.OwnerID, console.UpdateUserRequest{ProjectLimit: &newLimit})
				require.NoError(t, err)

				pr, err := sat.AddProject(userCtx1, up1Proj.OwnerID, "config test")
				require.NoError(t, err)
				require.NotNil(t, pr)
				require.Equal(t, pr.DefaultVersioning, console.Unversioned)

				salt, err := sat.DB.Console().Projects().GetSalt(ctx, pr.ID)
				require.NoError(t, err)
				require.NotNil(t, salt)

				// Getting project config as a non-member should not work
				config, err := service.GetProjectConfig(userCtx2, pr.ID)
				require.Error(t, err)
				require.Nil(t, config)

				// Getting project config as owner should work
				config, err = service.GetProjectConfig(userCtx1, pr.ID)
				require.NoError(t, err)
				require.NotNil(t, config)
				require.True(t, config.IsOwnerPaidTier)
				require.True(t, config.HasPaidPrivileges)
				require.Equal(t, console.RoleAdmin, config.Role)
				require.Equal(t, base64.StdEncoding.EncodeToString(salt), config.Salt)

				// add userCtx2 as member
				member, err := service.GetUser(ctx, up2Proj.OwnerID)
				require.NoError(t, err)
				require.Equal(t, console.FreeUser, member.Kind)

				_, err = service.AddProjectMembers(userCtx1, pr.ID, []string{member.Email})
				require.NoError(t, err)

				config, err = service.GetProjectConfig(userCtx2, pr.ID)
				require.NoError(t, err)
				require.Equal(t, console.RoleMember, config.Role)
				// member is not paid tier, but project owner is.
				require.True(t, config.IsOwnerPaidTier)
				require.True(t, config.HasPaidPrivileges)
				require.EqualValues(t, 2, config.MembersCount)

				kind := console.NFRUser
				err = sat.DB.Console().Users().Update(ctx, up1Proj.OwnerID, console.UpdateUserRequest{Kind: &kind})
				require.NoError(t, err)

				config, err = service.GetProjectConfig(userCtx1, pr.ID)
				require.NoError(t, err)
				require.NotNil(t, config)
				require.False(t, config.IsOwnerPaidTier)
				// nfr user has paid privileges
				require.True(t, config.HasPaidPrivileges)
				require.EqualValues(t, 2, config.MembersCount)

				token := "test-token"
				feats := entitlements.ProjectFeatures{ComputeAccessToken: []byte(token)}
				featBytes, err := json.Marshal(feats)
				require.NoError(t, err)

				_, err = sat.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
					Scope:    entitlements.ConvertPublicIDToProjectScope(pr.PublicID),
					Features: featBytes,
				})
				require.NoError(t, err)

				config, err = service.GetProjectConfig(userCtx1, pr.ID)
				require.NoError(t, err)
				require.NotNil(t, config)
				require.Equal(t, token, config.ComputeAuthToken)

				_, err = service.GetProjectConfig(userCtx1, disabledProject.ID)
				require.True(t, console.ErrUnauthorized.Has(err))

				kind = console.PaidUser
				err = sat.DB.Console().Users().Update(ctx, up1Proj.OwnerID, console.UpdateUserRequest{Kind: &kind})
				require.NoError(t, err)
			})
		})
}

func TestGetUsageReport(t *testing.T) {
	// use large prices so that we can test the calculations with small usages
	var (
		productID         = int32(1)
		productID2        = int32(2)
		placement11       = storj.PlacementConstraint(11)
		placementDetail11 = console.PlacementDetail{
			ID:     int(placement11),
			IdName: "placement11",
		}
		placement13       = storj.PlacementConstraint(13)
		placementDetail13 = console.PlacementDetail{
			ID:     int(placement13),
			IdName: "placement13",
		}
		skus = paymentsconfig.ProductSKUs{
			StorageSKU: "StorageSKU",
			EgressSKU:  "EgressSKU",
			SegmentSKU: "SegmentSKU",
		}
		defaultPrice = paymentsconfig.ProductUsagePrice{
			ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "300000",
				EgressTB:  "300000",
				Segment:   "3",
			},
		}
		productPrice = paymentsconfig.ProductUsagePrice{
			Name:        "product1",
			ProductSKUs: skus,
			ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "200000",
				EgressTB:  "200000",
				Segment:   "2",
			},
		}
		productPrice2 = paymentsconfig.ProductUsagePrice{
			Name:        "product2",
			ProductSKUs: skus,
			ProjectUsagePrice: paymentsconfig.ProjectUsagePrice{
				StorageTB: "100000",
				EgressTB:  "100000",
				Segment:   "1",
			},
		}
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Placement.SelfServeEnabled = true
				config.Console.NewDetailedUsageReportEnabled = true
				config.Payments.StripeCoinPayments.SkuEnabled = true
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "defaultPlacement");11:annotation("location", "placement11");13:annotation("location", "placement13")`,
				}
				config.Payments.UsagePrice = paymentsconfig.ProjectUsagePrice{
					StorageTB:           defaultPrice.StorageTB,
					EgressTB:            defaultPrice.EgressTB,
					Segment:             defaultPrice.Segment,
					EgressDiscountRatio: defaultPrice.EgressDiscountRatio,
				}
				config.Payments.Products.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					productID:  productPrice,
					productID2: productPrice2,
				})
				config.Payments.PlacementPriceOverrides.SetMap(map[int]int32{
					int(storj.DefaultPlacement): productID,
					int(placement11):            productID2,
				})
				config.Console.Placement.SelfServeDetails.SetMap(map[storj.PlacementConstraint]console.PlacementDetail{
					storj.DefaultPlacement: {ID: 0},
					placement11:            placementDetail11,
					placement13:            placementDetail13,
				})

				config.Entitlements.Enabled = true
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			now := time.Now().Truncate(time.Hour).UTC()
			var (
				sat                 = planet.Satellites[0]
				service             = sat.API.Console.Service
				projectID           = planet.Uplinks[0].Projects[0].ID
				endpoint            = sat.API.Metainfo.Endpoint
				projectEntitlements = sat.API.Entitlements.Service.Projects()
				db                  = sat.DB
			)

			sat.Accounting.Tally.Loop.Pause()
			sat.Accounting.Rollup.Loop.Pause()
			sat.Accounting.RollupArchive.Loop.Pause()

			project, err := db.Console().Projects().Get(ctx, projectID)
			require.NoError(t, err)

			// we only want to use one user and project for this test
			// the extra uplink and project is only to "contaminate" the
			// database with some data so that we can test the report
			// returned is correct for this user.
			user, err := db.Console().Users().Get(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
			require.NoError(t, err)

			usrCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			numbBuckets := 4

			createBucketWithUsage := func(projectID uuid.UUID, apiKey *macaroon.APIKey, bucket, placementName string) {
				_, err = endpoint.CreateBucket(ctx, &pb.CreateBucketRequest{
					Header:    &pb.RequestHeader{ApiKey: apiKey.SerializeRaw()},
					Name:      []byte(bucket),
					Placement: []byte(placementName),
				})
				require.NoError(t, err)

				err = generateRollups(ctx, db.Orders(), projectID, bucket, now)
				require.NoError(t, err)
				err = generateTallies(ctx, db.ProjectAccounting(), projectID, bucket, now)
				require.NoError(t, err)
				err = db.Attribution().UpdateUserAgent(ctx, projectID, bucket, []byte(strings.ReplaceAll(projectID.String(), "-", " ")))
				require.NoError(t, err)
			}

			bucketNames := make([]string, 0, numbBuckets)
			for _, upl := range planet.Uplinks {
				apiKey := upl.APIKey[sat.ID()]
				pID := upl.Projects[0].ID

				p, err := db.Console().Projects().Get(ctx, pID)
				require.NoError(t, err)

				err = projectEntitlements.SetNewBucketPlacementsByPublicID(ctx, p.PublicID,
					[]storj.PlacementConstraint{placement11, placement13, storj.DefaultPlacement},
				)
				require.NoError(t, err)

				for i := 0; i < numbBuckets; i++ {
					bucketName := fmt.Sprintf("bucket-%d", i)
					placementName := ""
					if i%2 == 0 {
						placementName = placementDetail11.IdName
					}
					createBucketWithUsage(pID, apiKey, bucketName, placementName)
					if pID == projectID {
						bucketNames = append(bucketNames, bucketName)
					}
				}
			}

			testCosts := func(item accounting.ProjectReportItem, rollup accounting.BucketUsageRollup) {
				_, priceModel := sat.API.Payments.Accounts.GetPartnerPlacementPriceModel(ctx, project.PublicID,
					string(project.UserAgent), item.Placement)
				require.Equal(t, priceModel.StorageSKU, item.StorageSKU)
				require.Equal(t, priceModel.EgressSKU, item.EgressSKU)
				require.Equal(t, priceModel.SegmentSKU, item.SegmentSKU)
				require.Equal(t, priceModel.ProductName, item.ProductName)

				// egress and storage are in GB, convert to bytes
				storageBytes, _ := decimal.NewFromFloat(rollup.TotalStoredData).Shift(9).Float64()
				egressBytes, _ := decimal.NewFromFloat(rollup.GetEgress).Shift(9).Float64()

				usage := accounting.ProjectUsage{
					Storage:      storageBytes,
					Egress:       int64(egressBytes),
					ObjectCount:  rollup.ObjectCount,
					SegmentCount: rollup.TotalSegments,
				}
				usageCost := sat.API.Payments.Accounts.CalculateProjectUsagePrice(usage, priceModel.ProjectUsagePriceModel)

				expectedStorageCost, _ := usageCost.Storage.Float64()
				expectedEgressCost, _ := usageCost.Egress.Float64()
				expectedSegmentCost, _ := usageCost.Segment.Float64()
				expectedTotalCost := expectedStorageCost + expectedEgressCost + expectedSegmentCost

				require.Equal(t, expectedStorageCost, item.StorageCost)
				require.Equal(t, expectedEgressCost, item.EgressCost)
				require.Equal(t, expectedSegmentCost, item.SegmentCost)
				require.Equal(t, expectedTotalCost, item.TotalCost)
			}

			params := console.GetUsageReportParam{
				Since:       now.Add(-24 * time.Hour),
				Before:      now.Add(time.Hour).UTC(),
				ProjectID:   projectID,
				IncludeCost: true,
			}

			rollupForItem := func(item accounting.ProjectReportItem) accounting.BucketUsageRollup {
				if item.BucketName != "" {
					rollup, err := db.ProjectAccounting().GetSingleBucketUsageRollup(ctx, projectID, item.BucketName, params.Since, params.Before)
					require.NoError(t, err)
					require.NotNil(t, rollup)
					return *rollup
				}

				rollups, err := db.ProjectAccounting().GetBucketUsageRollups(ctx, projectID, params.Since, params.Before, true)
				require.NoError(t, err)
				require.Len(t, rollups, numbBuckets)

				var sum *accounting.BucketUsageRollup
				for _, rollup := range rollups {
					if rollup.Placement == item.Placement {
						if sum != nil {
							sum.GetEgress += rollup.GetEgress
							sum.TotalSegments += rollup.TotalSegments
							sum.ObjectCount += rollup.ObjectCount
							sum.TotalStoredData += rollup.TotalStoredData
						} else {
							sum = &rollup
						}
					}
				}
				require.NotNil(t, sum)
				return *sum
			}

			items, err := service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, numbBuckets)
			for _, item := range items {
				require.NotEmpty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			params.ProjectID = uuid.UUID{}
			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, numbBuckets)
			for _, item := range items {
				require.NotEmpty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			params.GroupByProject = true
			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, 2) // 2 unique placements; products
			for _, item := range items {
				require.Empty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			// test that deleted bucketNames with usage is still included in the report
			for i, bucket := range bucketNames {
				if i%2 == 0 {
					continue
				}
				err = sat.API.Buckets.Service.DeleteBucket(usrCtx, []byte(bucket), projectID)
				require.NoError(t, err)
			}

			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, 2)
			for _, item := range items {
				// check that items are mapped to the product and pricing
				// defined globally
				if item.Placement == placement11 {
					require.Equal(t, productPrice2.Name, item.ProductName)
				}
				if item.Placement == storj.DefaultPlacement {
					require.Equal(t, productPrice.Name, item.ProductName)
				}
				require.Empty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			params.GroupByProject = false
			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, numbBuckets)
			for _, item := range items {
				// check that items are mapped to the product and pricing
				// defined globally
				if item.Placement == placement11 {
					require.Equal(t, productPrice2.Name, item.ProductName)
				}
				if item.Placement == storj.DefaultPlacement {
					require.Equal(t, productPrice.Name, item.ProductName)
				}
				require.NotEmpty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			// test overriding project price mappings via entitlement
			err = projectEntitlements.SetPlacementProductMappingsByPublicID(ctx, project.PublicID, entitlements.PlacementProductMappings{
				storj.DefaultPlacement: productID2, // reverse of the global mapping
				placement11:            productID,
			})
			require.NoError(t, err)

			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, numbBuckets)
			for _, item := range items {
				// expect that items are mapped to the opposite product and pricing now
				if item.Placement == placement11 {
					require.Equal(t, productPrice.Name, item.ProductName)
				}
				if item.Placement == storj.DefaultPlacement {
					require.Equal(t, productPrice2.Name, item.ProductName)
				}
				require.NotEmpty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			params.GroupByProject = true
			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, 2)
			for _, item := range items {
				// expect that items are mapped to the opposite product and pricing now
				if item.Placement == placement11 {
					require.Equal(t, productPrice.Name, item.ProductName)
				}
				if item.Placement == storj.DefaultPlacement {
					require.Equal(t, productPrice2.Name, item.ProductName)
				}
				require.Empty(t, item.BucketName)
				testCosts(item, rollupForItem(item))
			}

			// remove the entitlement mappings
			err = projectEntitlements.SetPlacementProductMappingsByPublicID(ctx, project.PublicID, entitlements.PlacementProductMappings{})
			require.NoError(t, err)

			apiKey := planet.Uplinks[0].APIKey[sat.ID()]
			pID := planet.Uplinks[0].Projects[0].ID
			// test that placement 13 without a specific product mapping
			// will not cause report to break, but will use the default pricing
			createBucketWithUsage(pID, apiKey, "bucket-placement-13", placementDetail13.IdName)

			params.GroupByProject = false
			items, err = service.GetUsageReport(usrCtx, params)
			require.NoError(t, err)
			require.Len(t, items, numbBuckets+1)
			for _, item := range items {
				if item.Placement == placement13 {
					_, priceModel := sat.API.Payments.Accounts.GetPartnerPlacementPriceModel(ctx, project.PublicID,
						string(project.UserAgent), item.Placement)
					defaultModel, err := defaultPrice.ToModel()
					require.NoError(t, err)
					require.Equal(t, defaultModel, priceModel.ProjectUsagePriceModel)
					testCosts(item, rollupForItem(item))
					break
				}
			}

			user, err = sat.AddUser(ctx, console.CreateUser{
				FullName:  "test_name",
				ShortName: "",
				Email:     "test@storj.test",
			}, 1)
			require.NoError(t, err)

			usrCtx2, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			params.ProjectID = projectID
			params.GroupByProject = false
			_, err = service.GetUsageReport(usrCtx2, params)
			require.True(t, console.ErrUnauthorized.Has(err))
		},
	)
}

func generateTallies(ctx *testcontext.Context, db accounting.ProjectAccounting, projectID uuid.UUID, bucket string, now time.Time) (err error) {
	bucketLoc1 := metabase.BucketLocation{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucket),
	}

	bucketTallies := make(map[metabase.BucketLocation]*accounting.BucketTally)
	for i := 1; i <= 10; i++ {
		tally1 := &accounting.BucketTally{
			BucketLocation: bucketLoc1,
			ObjectCount:    10000,
			TotalSegments:  10000,
			TotalBytes:     (100 * memory.TB).Int64(),
			MetadataSize:   (10 * memory.GB).Int64(),
		}
		bucketTallies[bucketLoc1] = tally1

		intervalPadding := -1 * (time.Duration(i) * time.Hour)
		err = db.SaveTallies(ctx, now.Add(intervalPadding), bucketTallies)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateRollups(ctx *testcontext.Context, db orders.DB, projectID uuid.UUID, bucket string, now time.Time) (err error) {
	actions := []pb.PieceAction{
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
	}
	rollups := make([]orders.BucketBandwidthRollup, 0)
	for _, action := range actions {
		rollups = append(rollups, orders.BucketBandwidthRollup{
			ProjectID:     projectID,
			BucketName:    bucket,
			Action:        action,
			IntervalStart: now.Add(-time.Hour * 2),
			Inline:        (30 * memory.GB).Int64(),
			Allocated:     (60 * memory.TB).Int64(),
			Settled:       (40 * memory.TB).Int64(),
		})

		rollups = append(rollups, orders.BucketBandwidthRollup{
			ProjectID:     projectID,
			BucketName:    bucket,
			Action:        action,
			IntervalStart: now,
			Inline:        (30 * memory.GB).Int64(),
			Allocated:     (60 * memory.TB).Int64(),
			Settled:       (40 * memory.TB).Int64(),
		})
	}
	err = db.UpdateBandwidthBatch(ctx, rollups)
	if err != nil {
		return err
	}
	return nil
}

func TestChangeEmail(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.EmailChangeFlowEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		service := sat.API.Console.Service
		usrLogin := planet.Uplinks[0].User[sat.ID()]

		user, _, err := service.GetUserByEmailWithUnverified(ctx, usrLogin.Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		updateContext := func() (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err := console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}
		userCtx, user := updateContext()

		// 2fa is disabled.
		err = service.ChangeEmail(userCtx, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)

		mfaSecret, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)

		now := time.Now()
		goodCode, err := console.NewMFAPasscode(mfaSecret, now)
		require.NoError(t, err)

		err = service.EnableUserMFA(userCtx, goodCode, now)
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.NotEmpty(t, user.MFASecretKey)
		require.Zero(t, user.EmailChangeVerificationStep)

		// starting from second step must fail.
		err = service.ChangeEmail(userCtx, console.VerifyAccountMfaStep, "test")
		require.True(t, console.ErrValidation.Has(err))

		userCtx, user = updateContext()
		require.Zero(t, user.EmailChangeVerificationStep)

		for i := 0; i < 2; i++ {
			err = service.ChangeEmail(userCtx, console.VerifyAccountPasswordStep, "wrong password")
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext()
		}

		// account gets locked after 3 failed attempts.
		err = service.ChangeEmail(userCtx, console.VerifyAccountPasswordStep, usrLogin.Password)
		require.True(t, console.ErrUnauthorized.Has(err))

		resetAccountLock := func() error {
			failedLoginCount := 0
			loginLockoutExpirationPtr := &time.Time{}

			return db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				FailedLoginCount:       &failedLoginCount,
				LoginLockoutExpiration: &loginLockoutExpirationPtr,
			})
		}

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, _ = updateContext()

		err = service.ChangeEmail(userCtx, console.VerifyAccountPasswordStep, usrLogin.Password)
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		wrongCode, err := console.NewMFAPasscode(mfaSecret, now.Add(time.Hour))
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			err = service.ChangeEmail(userCtx, console.VerifyAccountMfaStep, wrongCode)
			require.True(t, console.ErrMFAPasscode.Has(err))

			userCtx, _ = updateContext()
		}

		goodCode, err = console.NewMFAPasscode(mfaSecret, now)
		require.NoError(t, err)

		// account gets locked after 3 failed attempts.
		err = service.ChangeEmail(userCtx, console.VerifyAccountMfaStep, goodCode)
		require.True(t, console.ErrUnauthorized.Has(err))

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		err = service.ChangeEmail(userCtx, console.VerifyAccountMfaStep, goodCode)
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

		for i := 0; i < 3; i++ {
			err = service.ChangeEmail(userCtx, console.VerifyAccountEmailStep, "random verification code")
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext()
		}

		// account gets locked after 3 failed attempts.
		err = service.ChangeEmail(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
		require.True(t, console.ErrUnauthorized.Has(err))

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

		err = service.ChangeEmail(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)
		require.Empty(t, user.ActivationCode)

		err = service.ChangeEmail(userCtx, console.ChangeAccountEmailStep, "random string")
		require.True(t, console.ErrValidation.Has(err))

		anotherUsr, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test Change Email",
			Email:    "amother.usr@mail.test",
		}, 1)
		require.NoError(t, err)

		err = service.ChangeEmail(userCtx, console.ChangeAccountEmailStep, anotherUsr.Email)
		require.True(t, console.ErrValidation.Has(err))

		validEmail := "valid.email@mail.test"
		err = service.ChangeEmail(userCtx, console.ChangeAccountEmailStep, validEmail)
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.ChangeAccountEmailStep, user.EmailChangeVerificationStep)
		require.Equal(t, validEmail, *user.NewUnverifiedEmail)
		require.NotEmpty(t, user.ActivationCode)

		err = service.ChangeEmail(userCtx, console.ChangeAccountEmailStep, validEmail)
		require.True(t, console.ErrConflict.Has(err))

		for i := 0; i < 3; i++ {
			err = service.ChangeEmail(userCtx, console.VerifyNewAccountEmailStep, "random verification code")
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext()
		}

		// account gets locked after 3 failed attempts.
		err = service.ChangeEmail(userCtx, console.VerifyNewAccountEmailStep, user.ActivationCode)
		require.True(t, console.ErrUnauthorized.Has(err))

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, _ = updateContext()

		err = service.ChangeEmail(userCtx, console.VerifyNewAccountEmailStep, user.ActivationCode)
		require.NoError(t, err)

		_, user = updateContext()
		require.Equal(t, 0, user.EmailChangeVerificationStep)
		require.Nil(t, user.NewUnverifiedEmail)
		require.Equal(t, validEmail, user.Email)
		require.Empty(t, user.ActivationCode)

		// test sso user can't change email.
		ssoUser, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@storj.test",
			FullName: "test test",
		}, 1)
		require.NoError(t, err)
		require.NoError(t, service.UpdateExternalID(ctx, ssoUser, "test:1234"))

		ssoUserCtx, err := sat.UserContext(ctx, ssoUser.ID)
		require.NoError(t, err)

		err = service.ChangeEmail(ssoUserCtx, console.ChangeAccountEmailStep, "foobar")
		require.Error(t, err)
		require.True(t, console.ErrForbidden.Has(err))
		require.Contains(t, err.Error(), "sso")
	})
}

func TestCreateProject_WithEntitlementsService(t *testing.T) {
	var (
		placement10       = storj.PlacementConstraint(10)
		placement50       = storj.PlacementConstraint(50)
		placement10Detail = console.PlacementDetail{
			ID:     10,
			IdName: "placement10",
		}
		placement0Detail = console.PlacementDetail{
			ID:     0,
			IdName: "0",
		}
		allowedPlacements = []storj.PlacementConstraint{storj.DefaultPlacement, placement10}
		partnersMapping   = entitlements.PlacementProductMappings{
			storj.DefaultPlacement:        2,
			storj.PlacementConstraint(12): 2,
		}
		defaultMapping = entitlements.PlacementProductMappings{
			storj.DefaultPlacement:        1,
			storj.PlacementConstraint(12): 2,
		}
	)
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global");12:annotation("location", "archive")`,
				}
				config.Console.Placement.SelfServeEnabled = true
				config.Console.Placement.SelfServeDetails.SetMap(map[storj.PlacementConstraint]console.PlacementDetail{
					storj.DefaultPlacement: placement0Detail,
					placement10:            placement10Detail,
				})
				config.Console.Placement.AllowedPlacementIdsForNewProjects = allowedPlacements
				config.Entitlements.Enabled = true

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0: 1, 12: 2,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap

				var part1Map paymentsconfig.PlacementProductMap
				part1Map.SetMap(map[int]int32{
					0: 2,
				})
				partnersMap := make(map[string]paymentsconfig.PlacementProductMap)
				partnersMap["part1"] = part1Map
				config.Payments.PartnersPlacementPriceOverrides.SetMap(partnersMap)

				price := paymentsconfig.ProjectUsagePrice{
					StorageTB: "4",
					EgressTB:  "7",
					Segment:   "0.0000088",
				}
				var productOverrides paymentsconfig.ProductPriceOverrides
				productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					1: {ProjectUsagePrice: price},
					2: {ProjectUsagePrice: price},
				})
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "example@mail.test",
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		p, err := service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "test-project"})
		require.NoError(t, err)

		feats, err := sat.API.Entitlements.Service.Projects().GetByPublicID(ctx, p.PublicID)
		require.NoError(t, err)
		require.NotNil(t, feats.NewBucketPlacements)
		require.EqualValues(t, allowedPlacements, feats.NewBucketPlacements)
		require.NotNil(t, feats.PlacementProductMappings)
		require.EqualValues(t, defaultMapping, feats.PlacementProductMappings)
		require.Contains(t, feats.NewBucketPlacements, p.DefaultPlacement)

		config, err := service.GetProjectConfig(userCtx, p.ID)
		require.NoError(t, err)
		require.Equal(t, len(feats.NewBucketPlacements), len(config.AvailablePlacements))
		require.Contains(t, config.AvailablePlacements, placement10Detail)
		require.Contains(t, config.AvailablePlacements, placement0Detail)

		user, err = sat.AddUser(ctx, console.CreateUser{
			FullName:  "Test User2",
			Email:     "example2@mail.test",
			UserAgent: []byte("part1"),
		}, 1)
		require.NoError(t, err)

		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		p, err = service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "test-project"})
		require.NoError(t, err)

		feats, err = sat.API.Entitlements.Service.Projects().GetByPublicID(ctx, p.PublicID)
		require.NoError(t, err)
		require.NotNil(t, feats.NewBucketPlacements)
		require.EqualValues(t, allowedPlacements, feats.NewBucketPlacements)
		require.NotNil(t, feats.PlacementProductMappings)
		require.EqualValues(t, partnersMapping, feats.PlacementProductMappings)
		require.Contains(t, feats.NewBucketPlacements, p.DefaultPlacement)

		config, err = service.GetProjectConfig(userCtx, p.ID)
		require.NoError(t, err)
		require.Equal(t, len(feats.NewBucketPlacements), len(config.AvailablePlacements))
		require.Contains(t, config.AvailablePlacements, placement10Detail)
		require.Contains(t, config.AvailablePlacements, placement0Detail)

		user, err = sat.AddUser(ctx, console.CreateUser{
			FullName: "Non default placement User",
			Email:    "nondefaultplacement@mail.test",
		}, 1)
		require.NoError(t, err)

		err = sat.DB.Console().Users().UpdateDefaultPlacement(ctx, user.ID, placement50)
		require.NoError(t, err)

		user, err = service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, placement50, user.DefaultPlacement)

		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		p, err = service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "test-project"})
		require.NoError(t, err)
		require.Equal(t, placement50, p.DefaultPlacement)

		feats, err = sat.API.Entitlements.Service.Projects().GetByPublicID(ctx, p.PublicID)
		require.NoError(t, err)
		require.NotNil(t, feats.NewBucketPlacements)
		require.EqualValues(t, []storj.PlacementConstraint{placement50}, feats.NewBucketPlacements)
		require.NotNil(t, feats.PlacementProductMappings)
		require.EqualValues(t, defaultMapping, feats.PlacementProductMappings)

		config, err = service.GetProjectConfig(userCtx, p.ID)
		require.NoError(t, err)
		require.Empty(t, config.AvailablePlacements)

		// delete entitlements for the project
		err = sat.API.Entitlements.Service.Projects().DeleteByPublicID(ctx, p.PublicID)
		require.NoError(t, err)

		// still expect that project has no available placements
		config, err = service.GetProjectConfig(userCtx, p.ID)
		require.NoError(t, err)
		require.Empty(t, config.AvailablePlacements)
	})
}

func TestDeleteProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.DeleteProjectEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		service := sat.API.Console.Service
		uplinks := planet.Uplinks
		require.Len(t, uplinks, 2)

		usrLogin := uplinks[0].User[sat.ID()]
		user, _, err := service.GetUserByEmailWithUnverified(ctx, usrLogin.Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		updateContext := func() (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err := console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}

		uplink := uplinks[1]

		usrLogin = uplink.User[sat.ID()]
		user, _, err = service.GetUserByEmailWithUnverified(ctx, usrLogin.Email)
		require.NoError(t, err)
		require.NotNil(t, user)

		user.Kind = console.PaidUser
		require.NoError(t, db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &user.Kind}))

		require.Len(t, uplink.Projects, 1)
		p := uplink.Projects[0]

		userCtx, user := updateContext()

		// check resp contains buckets
		bucket := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket1",
			ProjectID: p.ID,
		}
		_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
		require.NoError(t, err)

		resp, err := service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.Equal(t, 1, resp.Buckets)

		require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p.ID))

		// check resp contains api keys
		resp, err = service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.Equal(t, resp.APIKeys, 1)

		keys, err := service.GetAllAPIKeyNamesByProjectID(userCtx, p.PublicID)
		require.NoError(t, err)
		require.Len(t, keys, 1)

		require.NoError(t, service.DeleteAPIKeyByNameAndProjectID(userCtx, keys[0], p.PublicID))

		// set time to middle of day to avoid usage being created in previous month
		// if this test runs early on the first day of the month
		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

		service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})
		interval := timestamp.Add(-2 * time.Hour)

		// check for unbilled storage
		// storage usage is calculated between two tally rows
		require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
			BucketName:    bucket.Name,
			ProjectID:     bucket.ProjectID,
			IntervalStart: interval,
			TotalBytes:    10000,
		}))

		interval = interval.Add(time.Hour)

		require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
			BucketName:    bucket.Name,
			ProjectID:     bucket.ProjectID,
			IntervalStart: interval,
			TotalBytes:    10000,
		}))

		resp, err = service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		// can't delete bucket storage tallies, so manually delete the project and create another one.
		require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
		p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project 2",
		})
		require.NoError(t, err)

		// check for unbilled bandwidth
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, 1000000, 0, interval))
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
		require.NoError(t, err)

		// check for usage in previous month, but invoice not generated yet
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		egress := int64(1000000)
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, egress, 0, lastMonth))

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.InvoicingIncomplete)

		thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
			ProjectID: p2.ID,
			Egress:    egress,
		}}, lastMonth, thisMonth))

		// 2fa is disabled.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.Nil(t, resp)

		mfaSecret, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)

		goodCode, err := console.NewMFAPasscode(mfaSecret, timestamp)
		require.NoError(t, err)

		err = service.EnableUserMFA(userCtx, goodCode, timestamp)
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.NotEmpty(t, user.MFASecretKey)
		require.Zero(t, user.EmailChangeVerificationStep)

		// skipping straight to last step fails.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.DeleteProjectStep, "")
		require.Error(t, err)
		require.True(t, console.ErrValidation.Has(err))
		require.Nil(t, resp)

		// starting from second step must fail.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.True(t, console.ErrValidation.Has(err))
		require.Nil(t, resp)

		userCtx, user = updateContext()
		require.Zero(t, user.EmailChangeVerificationStep)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, "wrong password")
		require.True(t, console.ErrValidation.Has(err))
		require.Nil(t, resp)

		userCtx, _ = updateContext()

		// account gets locked after 3 failed attempts.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, usrLogin.Password)
		require.True(t, console.ErrUnauthorized.Has(err))
		require.Nil(t, resp)

		resetAccountLock := func() error {
			failedLoginCount := 0
			loginLockoutExpirationPtr := &time.Time{}

			return db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				FailedLoginCount:       &failedLoginCount,
				LoginLockoutExpiration: &loginLockoutExpirationPtr,
			})
		}

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, _ = updateContext()

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, usrLogin.Password)
		require.NoError(t, err)
		require.Nil(t, resp)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		wrongCode, err := console.NewMFAPasscode(mfaSecret, timestamp.Add(time.Hour))
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, wrongCode)
			require.True(t, console.ErrMFAPasscode.Has(err))
			require.Nil(t, resp)

			userCtx, _ = updateContext()
		}

		goodCode, err = console.NewMFAPasscode(mfaSecret, timestamp)
		require.NoError(t, err)

		// account gets locked after 3 failed attempts.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, goodCode)
		require.True(t, console.ErrUnauthorized.Has(err))
		require.Nil(t, resp)

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, goodCode)
		require.NoError(t, err)
		require.Nil(t, resp)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

		for i := 0; i < 3; i++ {
			_, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, "random verification code")
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext()
		}

		// account gets locked after 3 failed attempts.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, user.ActivationCode)
		require.True(t, console.ErrUnauthorized.Has(err))
		require.Nil(t, resp)

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, user.ActivationCode)
		require.NoError(t, err)
		require.Nil(t, resp)

		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)
		require.Empty(t, user.ActivationCode)

		// check that creating a bucket in between steps interrupts deletion
		bucket.ProjectID = p2.ID
		_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
		require.NoError(t, err)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, user.ActivationCode)
		require.Error(t, err)
		require.NotNil(t, resp)
		require.Equal(t, 1, resp.Buckets)

		require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p2.ID))

		// project deletion is successful
		project, err := sat.API.DB.Console().Projects().Get(ctx, p2.ID)
		require.NoError(t, err)
		require.NotNil(t, project)

		scope := entitlements.ConvertPublicIDToProjectScope(project.PublicID)
		feats := entitlements.ProjectFeatures{NewBucketPlacements: []storj.PlacementConstraint{storj.DefaultPlacement}}
		featBytes, err := json.Marshal(feats)
		require.NoError(t, err)
		require.NotNil(t, featBytes)

		_, err = sat.API.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
			Scope:    scope,
			Features: featBytes,
		})
		require.NoError(t, err)

		entitlement, err := sat.API.DB.Console().Entitlements().GetByScope(ctx, scope)
		require.NoError(t, err)
		require.NotNil(t, entitlement)
		require.NotNil(t, entitlement.Features)

		_, err = sat.DB.Console().Domains().Create(ctx, console.Domain{
			Subdomain:       "example.test",
			ProjectID:       p2.ID,
			ProjectPublicID: p2.PublicID,
			Prefix:          "example",
			AccessID:        "access-id",
			CreatedBy:       p2.OwnerID,
		})
		require.NoError(t, err)

		domains, err := sat.DB.Console().Domains().GetAllDomainNamesByProjectID(ctx, p2.ID)
		require.NoError(t, err)
		require.Len(t, domains, 1)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.DeleteProjectStep, "")
		require.NoError(t, err)
		require.Nil(t, resp)

		domains, err = sat.DB.Console().Domains().GetAllDomainNamesByProjectID(ctx, p2.ID)
		require.NoError(t, err)
		require.Len(t, domains, 0)

		_, err = sat.API.DB.Console().Entitlements().GetByScope(ctx, scope)
		require.True(t, entitlements.ErrNotFound.Has(err))

		projects, err := db.Console().Projects().GetOwnActive(ctx, user.ID)
		require.NoError(t, err)
		require.Zero(t, len(projects))

		// test sso user can't delete project
		ssoUser, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@sso.test",
			FullName: "test test",
			Kind:     console.PaidUser,
		}, 1)
		require.NoError(t, err)
		require.NoError(t, service.UpdateExternalID(ctx, ssoUser, "test:1234"))

		ssoUserCtx, err := sat.UserContext(ctx, ssoUser.ID)
		require.NoError(t, err)

		project, err = service.CreateProject(ssoUserCtx, console.UpsertProjectInfo{
			Name:        "test",
			Description: "desc",
		})
		require.NoError(t, err)
		require.NotNil(t, project)

		_, err = service.DeleteProject(ssoUserCtx, project.ID, console.DeleteAccountInit, "foobar")
		require.Error(t, err)
		require.True(t, console.ErrForbidden.Has(err))
		require.Contains(t, err.Error(), "sso")
	})
}

func TestDeleteProject_WithDeleteThreshold(t *testing.T) {
	// This tests that a user with usage less than the minimum fee can still
	// delete their project (if other requirements are met).
	usagePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "100000",
		EgressTB:  "100000",
		Segment:   "100000",
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.DeleteProjectEnabled = true
				config.Payments.DeleteProjectCostThreshold = 5
				config.Payments.UsagePrice = usagePrice
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "example1@mail.test",
			Kind:     console.PaidUser,
		}, 1)
		require.NoError(t, err)

		p, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		// set time to middle of day to avoid usage being created in previous month
		// if this test runs early on the first day of the month
		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

		service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})
		interval := timestamp.Add(-2 * time.Hour)

		addUsage := func(projectID uuid.UUID, size memory.Size) {
			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))

			interval = interval.Add(time.Hour)

			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))
		}

		addUsage(p.ID, 400*memory.MB) // large usage that should be more than the minimum fee

		resp, err := service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		// can't delete bucket storage tallies, so manually delete the project and create another one.
		require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
		p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project 2",
		})
		require.NoError(t, err)

		addUsage(p2.ID, memory.MB) // small usage that should be less than the minimum fee

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.Nil(t, resp)

		_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
		require.NoError(t, err)

		// check for usage in previous month, but invoice not generated yet
		// does not affect deletion of free users.
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		egress := int64(1000000)
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, p2.ID.Bytes(), pb.PieceAction_GET, egress, 0, lastMonth.Add(time.Hour)))

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.InvoicingIncomplete)
	})
}

// Mostly the same tests as TestDeleteProject, but with abbreviated deletion enabled.
// Abbreviated deletion skips checks for buckets, api keys. And marks the project
// as Pending Deletion at the end of the flow.
func TestAbbreviatedDeleteProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.DeleteProjectEnabled = true
				config.Console.AbbreviatedDeleteProjectEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		service := sat.API.Console.Service

		updateContext := func(user *console.User) (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err = console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}

		userLogin := planet.Uplinks[0].User[sat.ID()]

		user, err := db.Console().Users().GetByEmailAndTenant(ctx, userLogin.Email, nil)
		require.NoError(t, err)

		user.Kind = console.PaidUser
		require.NoError(t, db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &user.Kind}))

		userCtx, user := updateContext(user)

		p, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		bucket := buckets.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket1",
			ProjectID: p.ID,
		}
		_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
		require.NoError(t, err)

		resp, err := service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		// abbreviated deletion does not check for buckets
		require.NoError(t, err)
		require.Nil(t, resp)
		require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p.ID))

		bucket = buckets.Bucket{
			ID:         testrand.UUID(),
			Name:       "testBucket2",
			ProjectID:  p.ID,
			ObjectLock: buckets.ObjectLockSettings{Enabled: true},
		}
		_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
		require.NoError(t, err)

		resp, err = service.DeleteProject(userCtx, p.ID, console.DeleteProjectInit, "test")
		// abbreviated deletion does checks for object lock buckets
		require.Error(t, err)
		require.NotNil(t, resp)
		require.Equal(t, 1, resp.LockEnabledBuckets)

		require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p.ID))

		_, _, err = service.CreateAPIKey(userCtx, p.ID, "testKey", macaroon.APIKeyVersionMin)
		require.NoError(t, err)

		resp, err = service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		// abbreviated deletion does not check for api keys
		require.NoError(t, err)
		require.Nil(t, resp)
		require.NoError(t, service.DeleteAPIKeyByNameAndProjectID(userCtx, "testKey", p.PublicID))

		// set time to middle of day to avoid usage being created in previous month
		// if this test runs early on the first day of the month
		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

		service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})
		interval := timestamp.Add(-2 * time.Hour)

		// check for unbilled storage
		// storage usage is calculated between two tally rows
		require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
			BucketName:    bucket.Name,
			ProjectID:     bucket.ProjectID,
			IntervalStart: interval,
			TotalBytes:    10000,
		}))

		interval = interval.Add(time.Hour)

		require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
			BucketName:    bucket.Name,
			ProjectID:     bucket.ProjectID,
			IntervalStart: interval,
			TotalBytes:    10000,
		}))

		resp, err = service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		// can't delete bucket storage tallies, so manually delete the project and create another one.
		require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
		p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project 2",
		})
		require.NoError(t, err)

		// check for unbilled bandwidth
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, 1000000, 0, interval))
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
		require.NoError(t, err)

		// check for usage in previous month, but invoice not generated yet
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		egress := int64(1000000)
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, egress, 0, lastMonth))

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.InvoicingIncomplete)

		thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
			ProjectID: p2.ID,
			Egress:    egress,
		}}, lastMonth, thisMonth))

		// 2fa is disabled.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.Nil(t, resp)

		mfaSecret, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)

		goodCode, err := console.NewMFAPasscode(mfaSecret, timestamp)
		require.NoError(t, err)

		err = service.EnableUserMFA(userCtx, goodCode, timestamp)
		require.NoError(t, err)

		userCtx, user = updateContext(user)
		require.NotEmpty(t, user.MFASecretKey)
		require.Zero(t, user.EmailChangeVerificationStep)

		// skipping straight to last step fails.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.DeleteProjectStep, "")
		require.Error(t, err)
		require.True(t, console.ErrValidation.Has(err))
		require.Nil(t, resp)

		// starting from second step must fail.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.True(t, console.ErrValidation.Has(err))
		require.Nil(t, resp)

		userCtx, user = updateContext(user)
		require.Zero(t, user.EmailChangeVerificationStep)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, "wrong password")
		require.True(t, console.ErrValidation.Has(err))
		require.Nil(t, resp)

		userCtx, _ = updateContext(user)

		// account gets locked after 3 failed attempts.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, userLogin.Password)
		require.True(t, console.ErrUnauthorized.Has(err))
		require.Nil(t, resp)

		resetAccountLock := func() error {
			failedLoginCount := 0
			loginLockoutExpirationPtr := &time.Time{}

			return db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				FailedLoginCount:       &failedLoginCount,
				LoginLockoutExpiration: &loginLockoutExpirationPtr,
			})
		}

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, _ = updateContext(user)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, userLogin.Password)
		require.NoError(t, err)
		require.Nil(t, resp)

		userCtx, user = updateContext(user)
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		wrongCode, err := console.NewMFAPasscode(mfaSecret, timestamp.Add(time.Hour))
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, wrongCode)
			require.True(t, console.ErrMFAPasscode.Has(err))
			require.Nil(t, resp)

			userCtx, _ = updateContext(user)
		}

		goodCode, err = console.NewMFAPasscode(mfaSecret, timestamp)
		require.NoError(t, err)

		// account gets locked after 3 failed attempts.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, goodCode)
		require.True(t, console.ErrUnauthorized.Has(err))
		require.Nil(t, resp)

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, user = updateContext(user)
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, goodCode)
		require.NoError(t, err)
		require.Nil(t, resp)

		userCtx, user = updateContext(user)
		require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

		for i := 0; i < 3; i++ {
			_, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, "random verification code")
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext(user)
		}

		// account gets locked after 3 failed attempts.
		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, user.ActivationCode)
		require.True(t, console.ErrUnauthorized.Has(err))
		require.Nil(t, resp)

		err = resetAccountLock()
		require.NoError(t, err)

		userCtx, user = updateContext(user)
		require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, user.ActivationCode)
		require.NoError(t, err)
		require.Nil(t, resp)

		userCtx, user = updateContext(user)
		require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)
		require.Empty(t, user.ActivationCode)

		resp, err = service.DeleteProject(userCtx, p2.ID, console.DeleteProjectStep, "")
		require.NoError(t, err)
		require.Nil(t, resp)

		// project deletion is successful
		p2, err = db.Console().Projects().Get(ctx, p2.ID)
		require.NoError(t, err)
		require.NotNil(t, p2.Status)
		require.Equal(t, console.ProjectPendingDeletion, *p2.Status)

		// test sso user can't delete project
		ssoUser, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@sso.test",
			FullName: "test test",
			Kind:     console.PaidUser,
		}, 1)
		require.NoError(t, err)
		require.NoError(t, service.UpdateExternalID(ctx, ssoUser, "test:1234"))

		ssoUserCtx, err := sat.UserContext(ctx, ssoUser.ID)
		require.NoError(t, err)

		project, err := service.CreateProject(ssoUserCtx, console.UpsertProjectInfo{
			Name:        "test",
			Description: "desc",
		})
		require.NoError(t, err)
		require.NotNil(t, project)

		_, err = service.DeleteProject(ssoUserCtx, project.ID, console.DeleteAccountInit, "foobar")
		require.Error(t, err)
		require.True(t, console.ErrForbidden.Has(err))
		require.Contains(t, err.Error(), "sso")
	})
}

// Mostly the same tests as TestDeleteProject_WithDeleteThreshold but with abbreviated deletion enabled.
func TestAbbreviatedDeleteProject_WithDeleteThreshold(t *testing.T) {
	// This tests that a user with usage less than the minimum fee can still
	// delete their project (if other requirements are met).
	usagePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "100000",
		EgressTB:  "100000",
		Segment:   "100000",
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.DeleteProjectEnabled = true
				config.Console.AbbreviatedDeleteProjectEnabled = true
				config.Payments.DeleteProjectCostThreshold = 5
				config.Payments.UsagePrice = usagePrice
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		uplinkUser := planet.Uplinks[0].User[sat.ID()]
		user, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, uplinkUser.Email, nil)
		require.NoError(t, err)

		user.Kind = console.PaidUser

		err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			Kind: &user.Kind,
		})
		require.NoError(t, err)

		updateContext := func() (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err := console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}

		userCtx, user := updateContext()

		addUsage := func(projectID uuid.UUID, size memory.Size, interval time.Time) {
			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))

			interval = interval.Add(time.Hour)

			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))
		}

		p, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		// set time to middle of day to avoid usage being created in previous month
		// if this test runs early on the first day of the month
		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

		service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})
		interval := timestamp.Add(-2 * time.Hour)

		addUsage(p.ID, 400*memory.MB, interval) // large usage that should be more than the minimum fee

		resp, err := service.DeleteProject(userCtx, p.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		// can't delete bucket storage tallies, so manually delete the project and create another one.
		require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
		p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project 2",
		})
		require.NoError(t, err)

		addUsage(p2.ID, memory.MB, interval) // small usage that should be less than the minimum fee

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.Nil(t, resp)

		_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
		require.NoError(t, err)

		// check for usage in previous month, but invoice not generated yet
		// does not affect deletion of free users.
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		egress := int64(1000000)
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, p2.ID.Bytes(), pb.PieceAction_GET, egress, 0, lastMonth.Add(time.Hour)))

		resp, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountMfaStep, "test")
		require.Error(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.InvoicingIncomplete)

		thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
			ProjectID: p2.ID,
			Egress:    egress,
		}}, lastMonth, thisMonth))

		_, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountPasswordStep, uplinkUser.Password)
		require.NoError(t, err)
		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		_, err = service.DeleteProject(userCtx, p2.ID, console.VerifyAccountEmailStep, user.ActivationCode)
		require.NoError(t, err)
		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)

		_, err = service.DeleteProject(userCtx, p2.ID, console.DeleteProjectStep, "test")
		require.NoError(t, err)

		p2, err = sat.DB.Console().Projects().Get(userCtx, p2.ID)
		require.NoError(t, err)
		require.Equal(t, console.ProjectPendingDeletion, *p2.Status)
	})
}

func TestDeleteAccount(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SelfServeAccountDeleteEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		service := sat.API.Console.Service
		uplinks := planet.Uplinks
		require.Len(t, uplinks, 2)

		// pause rollup archive loop because some of the tests insert bandwidth rollups during time periods the loop would clean up
		sat.Accounting.RollupArchive.Loop.Pause()

		for i, uplink := range uplinks {
			usrLogin := uplink.User[sat.ID()]
			user, _, err := service.GetUserByEmailWithUnverified(ctx, usrLogin.Email)
			require.NoError(t, err)
			require.NotNil(t, user)

			// ensure one user is paid tier
			if i != 0 {
				user.Kind = console.PaidUser
				require.NoError(t, db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &user.Kind}))
			}

			require.Len(t, uplink.Projects, 1)
			p := uplink.Projects[0]

			// error if user is under legal hold
			status := console.LegalHold
			err = db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &status})
			require.NoError(t, err)

			updateContext := func() (context.Context, *console.User) {
				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)
				user, err := console.GetUser(userCtx)
				require.NoError(t, err)
				return userCtx, user
			}
			userCtx, user := updateContext()

			resp, err := service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.Nil(t, resp)
			require.True(t, console.ErrForbidden.Has(err))

			status = console.Active
			err = db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &status})
			require.NoError(t, err)

			userCtx, _ = updateContext()

			// check resp contains buckets
			bucket := buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      "testBucket1",
				ProjectID: p.ID,
			}
			_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, 1, resp.Buckets)

			require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p.ID))

			// check resp contains api keys
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, resp.ApiKeys, 1)

			keys, err := service.GetAllAPIKeyNamesByProjectID(userCtx, p.PublicID)
			require.NoError(t, err)
			require.Len(t, keys, 1)

			require.NoError(t, service.DeleteAPIKeyByNameAndProjectID(userCtx, keys[0], p.PublicID))

			// check for unpaid invoices (open or draft).
			// N.B. we no longer create invoices for free tier users, so technically this should be unnecessary in that case,
			// but it seems better to check than not.
			amountOwed := int64(1000)
			invoice1, err := sat.API.Payments.Accounts.Invoices().Create(ctx, user.ID, amountOwed, "open invoice")
			require.NoError(t, err)
			invoice2, err := sat.API.Payments.Accounts.Invoices().Create(ctx, user.ID, amountOwed, "draft invoice")
			require.NoError(t, err)

			_, err = sat.API.Payments.StripeClient.Invoices().FinalizeInvoice(invoice1.ID, nil)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, 2, resp.UnpaidInvoices)
			require.Equal(t, 2*amountOwed, resp.AmountOwed)

			_, err = sat.API.Payments.Accounts.Invoices().Delete(ctx, invoice1.ID)
			require.NoError(t, err)
			_, err = sat.API.Payments.Accounts.Invoices().Delete(ctx, invoice2.ID)
			require.NoError(t, err)

			// set time to middle of day to avoid usage being created in previous month
			// if this test runs early on the first day of the month
			year, month, day := time.Now().UTC().Date()
			timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

			newNow := func() time.Time { return timestamp }
			service.TestSetNow(newNow)
			sat.DB.Console().Users().TestSetNow(newNow)
			sat.API.Payments.StripeService.SetNow(newNow)
			interval := timestamp.Add(-2 * time.Hour)

			// check for unbilled storage
			// storage usage is calculated between two tally rows
			// does not affect deletion of free users.
			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    bucket.Name,
				ProjectID:     bucket.ProjectID,
				IntervalStart: interval,
				TotalBytes:    10000,
			}))

			interval = interval.Add(time.Hour)

			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    bucket.Name,
				ProjectID:     bucket.ProjectID,
				IntervalStart: interval,
				TotalBytes:    10000,
			}))

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			if user.IsPaid() {
				require.NotNil(t, resp)
				require.True(t, resp.CurrentUsage)
			} else {
				require.Nil(t, resp)
			}

			// can't delete bucket storage tallies, so delete the project and create another one.
			require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
			p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
				Name: "test project 2",
			})
			require.NoError(t, err)

			// check for unbilled bandwidth
			// does not affect deletion of free users.
			require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, 1000000, 0, interval))
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			if user.IsPaid() {
				require.NotNil(t, resp)
				require.True(t, resp.CurrentUsage)
			} else {
				require.Nil(t, resp)
			}

			_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
			require.NoError(t, err)

			// check for usage in previous month, but invoice not generated yet
			// does not affect deletion of free users.
			lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
			egress := int64(1000000)
			require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, egress, 0, lastMonth.Add(time.Hour)))

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			if user.IsPaid() {
				require.NotNil(t, resp)
				require.True(t, resp.InvoicingIncomplete)
			} else {
				require.Nil(t, resp)
			}

			thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
			require.NoError(t, sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
				ProjectID: p2.ID,
				Egress:    egress,
			}}, lastMonth, thisMonth))

			// 2fa is disabled.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			require.Nil(t, resp)

			mfaSecret, err := service.ResetMFASecretKey(userCtx)
			require.NoError(t, err)

			goodCode, err := console.NewMFAPasscode(mfaSecret, timestamp)
			require.NoError(t, err)

			err = service.EnableUserMFA(userCtx, goodCode, timestamp)
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.NotEmpty(t, user.MFASecretKey)
			require.Zero(t, user.EmailChangeVerificationStep)

			// starting from second step must fail.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.True(t, console.ErrValidation.Has(err))
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Zero(t, user.EmailChangeVerificationStep)

			for i := 0; i < 2; i++ {
				resp, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, "wrong password")
				require.True(t, console.ErrValidation.Has(err))
				require.Nil(t, resp)

				userCtx, _ = updateContext()
			}

			// account gets locked after 3 failed attempts.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, usrLogin.Password)
			require.True(t, console.ErrUnauthorized.Has(err))
			require.Nil(t, resp)

			resetAccountLock := func() error {
				failedLoginCount := 0
				loginLockoutExpirationPtr := &time.Time{}

				return db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
					FailedLoginCount:       &failedLoginCount,
					LoginLockoutExpiration: &loginLockoutExpirationPtr,
				})
			}

			err = resetAccountLock()
			require.NoError(t, err)

			userCtx, _ = updateContext()

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, usrLogin.Password)
			require.NoError(t, err)
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

			wrongCode, err := console.NewMFAPasscode(mfaSecret, timestamp.Add(time.Hour))
			require.NoError(t, err)

			for i := 0; i < 3; i++ {
				resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, wrongCode)
				require.True(t, console.ErrMFAPasscode.Has(err))
				require.Nil(t, resp)

				userCtx, _ = updateContext()
			}

			goodCode, err = console.NewMFAPasscode(mfaSecret, timestamp)
			require.NoError(t, err)

			// account gets locked after 3 failed attempts.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, goodCode)
			require.True(t, console.ErrUnauthorized.Has(err))
			require.Nil(t, resp)

			err = resetAccountLock()
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, goodCode)
			require.NoError(t, err)
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

			for i := 0; i < 3; i++ {
				_, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, "random verification code")
				require.True(t, console.ErrValidation.Has(err))

				userCtx, _ = updateContext()
			}

			// account gets locked after 3 failed attempts.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
			require.True(t, console.ErrUnauthorized.Has(err))
			require.Nil(t, resp)

			err = resetAccountLock()
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
			require.NoError(t, err)
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)
			require.Empty(t, user.ActivationCode)

			// check that creating a bucket in between steps interrupts deletion
			bucket.ProjectID = p2.ID
			_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, 1, resp.Buckets)

			user, err = sat.API.DB.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.NotContains(t, user.Email, "deactivated")

			require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p2.ID))

			_, err = service.GenerateSessionToken(ctx, console.SessionTokenRequest{
				UserID:          user.ID,
				Email:           user.Email,
				IP:              "",
				UserAgent:       "",
				AnonymousID:     "",
				CustomDuration:  nil,
				HubspotObjectID: user.HubspotObjectID,
			})
			require.NoError(t, err)

			sessions, err := sat.DB.Console().WebappSessions().GetAllByUserID(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, sessions, 1)

			_, err = sat.API.Payments.Accounts.CreditCards().Add(ctx, user.ID, "testcard")
			require.NoError(t, err)

			cards, err := sat.API.Payments.Accounts.CreditCards().List(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, cards, 1)

			scope := entitlements.ConvertPublicIDToProjectScope(p2.PublicID)
			feats := entitlements.ProjectFeatures{NewBucketPlacements: []storj.PlacementConstraint{storj.DefaultPlacement}}
			featBytes, err := json.Marshal(feats)
			require.NoError(t, err)
			require.NotNil(t, featBytes)

			_, err = sat.API.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
				Scope:    scope,
				Features: featBytes,
			})
			require.NoError(t, err)

			entitlement, err := sat.API.DB.Console().Entitlements().GetByScope(ctx, scope)
			require.NoError(t, err)
			require.NotNil(t, entitlement)
			require.NotNil(t, entitlement.Features)

			_, err = sat.DB.Console().Domains().Create(ctx, console.Domain{
				Subdomain:       "example.test",
				ProjectID:       p2.ID,
				ProjectPublicID: p2.PublicID,
				Prefix:          "example",
				AccessID:        "access-id",
				CreatedBy:       p2.OwnerID,
			})
			require.NoError(t, err)

			domains, err := sat.DB.Console().Domains().GetAllDomainNamesByProjectID(ctx, p2.ID)
			require.NoError(t, err)
			require.Len(t, domains, 1)

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountStep, "")
			require.NoError(t, err)
			require.Nil(t, resp)

			_, user = updateContext()
			require.Equal(t, 0, user.EmailChangeVerificationStep)
			require.Equal(t, console.Deleted, user.Status)
			require.WithinDuration(t, timestamp, *user.StatusUpdatedAt, time.Minute)
			require.Empty(t, user.ActivationCode)
			require.Contains(t, user.Email, "deactivated")

			_, err = sat.API.DB.Console().Entitlements().GetByScope(ctx, scope)
			require.True(t, entitlements.ErrNotFound.Has(err))

			domains, err = sat.DB.Console().Domains().GetAllDomainNamesByProjectID(ctx, p2.ID)
			require.NoError(t, err)
			require.Len(t, domains, 0)

			projects, err := db.Console().Projects().GetOwnActive(ctx, user.ID)
			require.NoError(t, err)
			require.Zero(t, len(projects))

			// check credit cards
			cards, err = sat.API.Payments.Accounts.CreditCards().List(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, cards, 0)

			// check web sessions
			sessions, err = sat.DB.Console().WebappSessions().GetAllByUserID(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, sessions, 0)
		}

		// test sso user can't delete account
		ssoUser, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@sso.test",
			FullName: "test test",
		}, 1)
		require.NoError(t, err)
		require.NoError(t, service.UpdateExternalID(ctx, ssoUser, "test:1234"))

		ssoUserCtx, err := sat.UserContext(ctx, ssoUser.ID)
		require.NoError(t, err)
		_, err = service.DeleteAccount(ssoUserCtx, console.DeleteAccountInit, "foobar")
		require.Error(t, err)
		require.True(t, console.ErrForbidden.Has(err))
		require.Contains(t, err.Error(), "sso")
	})
}

func TestDeleteAccount_WithDeleteThreshold(t *testing.T) {
	// This tests that a user with usage less than the minimum fee can still
	// delete their account (if other requirements are met).
	usagePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "100000",
		EgressTB:  "100000",
		Segment:   "100000",
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SelfServeAccountDeleteEnabled = true
				config.Payments.DeleteProjectCostThreshold = 5
				config.Payments.UsagePrice = usagePrice
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "example1@mail.test",
			Kind:     console.PaidUser,
		}, 1)
		require.NoError(t, err)

		p, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		// set time to middle of day to avoid usage being created in previous month
		// if this test runs early on the first day of the month
		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

		service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})
		interval := timestamp.Add(-2 * time.Hour)

		addUsage := func(projectID uuid.UUID, size memory.Size) {
			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))

			interval = interval.Add(time.Hour)

			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))
		}

		addUsage(p.ID, 400*memory.MB) // large usage that should be more than the minimum fee

		resp, err := service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		// can't delete bucket storage tallies, so manually delete the project and create another one.
		require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
		p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project 2",
		})
		require.NoError(t, err)

		addUsage(p2.ID, memory.MB) // small usage that should be less than the minimum fee

		resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.Nil(t, resp)

		_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
		require.NoError(t, err)

		// check for usage in previous month, but invoice not generated yet
		// does not affect deletion of free users.
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		egress := int64(1000000)
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, p2.ID.Bytes(), pb.PieceAction_GET, egress, 0, lastMonth.Add(time.Hour)))

		resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.InvoicingIncomplete)
	})
}

// Mostly the same tests as TestDeleteAccount but with abbreviated deletion enabled.
// Abbreviated deletion skips checks for buckets, api keys. And marks the user
// as pending deletion after authentication steps are complete.
func TestAbbreviatedDeleteAccount(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SelfServeAccountDeleteEnabled = true
				config.Console.AbbreviatedDeleteAccountEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		service := sat.API.Console.Service
		uplinks := planet.Uplinks
		require.Len(t, uplinks, 2)

		// pause rollup archive loop because some of the tests insert bandwidth rollups during time periods the loop would clean up
		sat.Accounting.RollupArchive.Loop.Pause()

		for i, uplink := range uplinks {
			usrLogin := uplink.User[sat.ID()]
			user, _, err := service.GetUserByEmailWithUnverified(ctx, usrLogin.Email)
			require.NoError(t, err)
			require.NotNil(t, user)

			// ensure one user is paid tier
			if i != 0 {
				user.Kind = console.PaidUser
				require.NoError(t, db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &user.Kind}))
			}

			require.Len(t, uplink.Projects, 1)
			p := uplink.Projects[0]

			// error if user is under legal hold
			status := console.LegalHold
			err = db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &status})
			require.NoError(t, err)

			updateContext := func() (context.Context, *console.User) {
				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)
				user, err := console.GetUser(userCtx)
				require.NoError(t, err)
				return userCtx, user
			}
			userCtx, user := updateContext()

			resp, err := service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
			require.Nil(t, resp)
			require.True(t, console.ErrForbidden.Has(err))

			status = console.Active
			err = db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &status})
			require.NoError(t, err)

			userCtx, _ = updateContext()

			// check that having buckets does not interrupt
			// abbreviated deletion
			bucket := buckets.Bucket{
				ID:        testrand.UUID(),
				Name:      "testBucket1",
				ProjectID: p.ID,
			}
			_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
			require.NoError(t, err)
			require.Nil(t, resp)

			require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p.ID))

			// check that having object lock buckets interrupts deletion
			bucket = buckets.Bucket{
				ID:         testrand.UUID(),
				Name:       "testBucket2",
				ProjectID:  p.ID,
				ObjectLock: buckets.ObjectLockSettings{Enabled: true},
			}
			_, err = sat.API.Buckets.Service.CreateBucket(userCtx, bucket)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, 1, resp.LockEnabledBuckets)

			// object lock bucket check can be skipped
			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, console.SkipObjectLockEnabledBuckets)
			require.NoError(t, err)
			require.Nil(t, resp)

			// object lock bucket check only happens at initial step
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			require.Nil(t, resp)

			require.NoError(t, sat.API.Buckets.Service.DeleteBucket(ctx, []byte(bucket.Name), p.ID))

			// check that having api keys does not interrupt
			// abbreviated deletion
			_, _, err = service.CreateAPIKey(userCtx, p.PublicID, "testkey", macaroon.APIKeyVersionMin)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
			require.NoError(t, err)
			require.Nil(t, resp)

			// check for unpaid invoices (open or draft).
			// N.B. we no longer create invoices for free tier users, so technically this should be unnecessary in that case,
			// but it seems better to check than not.
			amountOwed := int64(1000)
			invoice1, err := sat.API.Payments.Accounts.Invoices().Create(ctx, user.ID, amountOwed, "open invoice")
			require.NoError(t, err)
			invoice2, err := sat.API.Payments.Accounts.Invoices().Create(ctx, user.ID, amountOwed, "draft invoice")
			require.NoError(t, err)

			_, err = sat.API.Payments.StripeClient.Invoices().FinalizeInvoice(invoice1.ID, nil)
			require.NoError(t, err)

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, 2, resp.UnpaidInvoices)
			require.Equal(t, 2*amountOwed, resp.AmountOwed)

			_, err = sat.API.Payments.Accounts.Invoices().Delete(ctx, invoice1.ID)
			require.NoError(t, err)
			_, err = sat.API.Payments.Accounts.Invoices().Delete(ctx, invoice2.ID)
			require.NoError(t, err)

			// set time to middle of day to avoid usage being created in previous month
			// if this test runs early on the first day of the month
			year, month, day := time.Now().UTC().Date()
			timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

			newNow := func() time.Time { return timestamp }
			service.TestSetNow(newNow)
			sat.DB.Console().Users().TestSetNow(newNow)
			sat.API.Payments.StripeService.SetNow(newNow)
			interval := timestamp.Add(-2 * time.Hour)

			// check for unbilled storage
			// storage usage is calculated between two tally rows
			// does not affect deletion of free users.
			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    bucket.Name,
				ProjectID:     bucket.ProjectID,
				IntervalStart: interval,
				TotalBytes:    10000,
			}))

			interval = interval.Add(time.Hour)

			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    bucket.Name,
				ProjectID:     bucket.ProjectID,
				IntervalStart: interval,
				TotalBytes:    10000,
			}))

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
			require.NoError(t, err)
			if user.IsPaid() {
				require.NotNil(t, resp)
				require.True(t, resp.CurrentUsage)
			} else {
				require.Nil(t, resp)
			}

			// can't delete bucket storage tallies, so delete the project and create another one.
			require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
			p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
				Name: "test project 2",
			})
			require.NoError(t, err)

			// check for unbilled bandwidth
			// does not affect deletion of free users.
			require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, 1000000, 0, interval))
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			if user.IsPaid() {
				require.NotNil(t, resp)
				require.True(t, resp.CurrentUsage)
			} else {
				require.Nil(t, resp)
			}

			_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
			require.NoError(t, err)

			// check for usage in previous month, but invoice not generated yet
			// does not affect deletion of free users.
			lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
			egress := int64(1000000)
			require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, []byte(bucket.Name), pb.PieceAction_GET, egress, 0, lastMonth.Add(time.Hour)))

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			if user.IsPaid() {
				require.NotNil(t, resp)
				require.True(t, resp.InvoicingIncomplete)
			} else {
				require.Nil(t, resp)
			}

			thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
			require.NoError(t, sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
				ProjectID: p2.ID,
				Egress:    egress,
			}}, lastMonth, thisMonth))

			// 2fa is disabled.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.NoError(t, err)
			require.Nil(t, resp)

			mfaSecret, err := service.ResetMFASecretKey(userCtx)
			require.NoError(t, err)

			goodCode, err := console.NewMFAPasscode(mfaSecret, timestamp)
			require.NoError(t, err)

			err = service.EnableUserMFA(userCtx, goodCode, timestamp)
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.NotEmpty(t, user.MFASecretKey)
			require.Zero(t, user.EmailChangeVerificationStep)

			// starting from second step must fail.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, "test")
			require.True(t, console.ErrValidation.Has(err))
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Zero(t, user.EmailChangeVerificationStep)

			for i := 0; i < 2; i++ {
				resp, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, "wrong password")
				require.True(t, console.ErrValidation.Has(err))
				require.Nil(t, resp)

				userCtx, _ = updateContext()
			}

			// account gets locked after 3 failed attempts.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, usrLogin.Password)
			require.True(t, console.ErrUnauthorized.Has(err))
			require.Nil(t, resp)

			resetAccountLock := func() error {
				failedLoginCount := 0
				loginLockoutExpirationPtr := &time.Time{}

				return db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
					FailedLoginCount:       &failedLoginCount,
					LoginLockoutExpiration: &loginLockoutExpirationPtr,
				})
			}

			err = resetAccountLock()
			require.NoError(t, err)

			userCtx, _ = updateContext()

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, usrLogin.Password)
			require.NoError(t, err)
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

			wrongCode, err := console.NewMFAPasscode(mfaSecret, timestamp.Add(time.Hour))
			require.NoError(t, err)

			for i := 0; i < 3; i++ {
				resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, wrongCode)
				require.True(t, console.ErrMFAPasscode.Has(err))
				require.Nil(t, resp)

				userCtx, _ = updateContext()
			}

			goodCode, err = console.NewMFAPasscode(mfaSecret, timestamp)
			require.NoError(t, err)

			// account gets locked after 3 failed attempts.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, goodCode)
			require.True(t, console.ErrUnauthorized.Has(err))
			require.Nil(t, resp)

			err = resetAccountLock()
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountMfaStep, goodCode)
			require.NoError(t, err)
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

			for i := 0; i < 3; i++ {
				_, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, "random verification code")
				require.True(t, console.ErrValidation.Has(err))

				userCtx, _ = updateContext()
			}

			// account gets locked after 3 failed attempts.
			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
			require.True(t, console.ErrUnauthorized.Has(err))
			require.Nil(t, resp)

			err = resetAccountLock()
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountMfaStep, user.EmailChangeVerificationStep)

			resp, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
			require.NoError(t, err)
			require.Nil(t, resp)

			userCtx, user = updateContext()
			require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)
			require.Empty(t, user.ActivationCode)

			resp, err = service.DeleteAccount(userCtx, console.DeleteAccountStep, "")
			require.NoError(t, err)
			require.Nil(t, resp)

			_, user = updateContext()
			require.Equal(t, 0, user.EmailChangeVerificationStep)
			require.Equal(t, console.PendingDeletion, user.Status)
			require.WithinDuration(t, timestamp, *user.StatusUpdatedAt, time.Minute)
			require.Empty(t, user.ActivationCode)

			// Abbreviated deletion does not delete related resources immediately
			projects, err := db.Console().Projects().GetOwnActive(ctx, user.ID)
			require.NoError(t, err)
			require.NotEmpty(t, projects)
		}

		// test sso user can't delete account
		ssoUser, err := sat.AddUser(ctx, console.CreateUser{
			Email:    "test@sso.test",
			FullName: "test test",
		}, 1)
		require.NoError(t, err)
		require.NoError(t, service.UpdateExternalID(ctx, ssoUser, "test:1234"))

		ssoUserCtx, err := sat.UserContext(ctx, ssoUser.ID)
		require.NoError(t, err)
		_, err = service.DeleteAccount(ssoUserCtx, console.DeleteAccountInit, "foobar")
		require.Error(t, err)
		require.True(t, console.ErrForbidden.Has(err))
		require.Contains(t, err.Error(), "sso")
	})
}

// Same as TestDeleteAccount_WithDeleteThreshold but for abbreviated deletion.
func TestAbbreviatedDeleteAccount_WithDeleteThreshold(t *testing.T) {
	// This tests that a user with usage less than the minimum fee can still
	// delete their account (if other requirements are met).
	usagePrice := paymentsconfig.ProjectUsagePrice{
		StorageTB: "100000",
		EgressTB:  "100000",
		Segment:   "100000",
	}
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SelfServeAccountDeleteEnabled = true
				config.Console.AbbreviatedDeleteAccountEnabled = true
				config.Payments.DeleteProjectCostThreshold = 5
				config.Payments.UsagePrice = usagePrice
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		userLogin := planet.Uplinks[0].User[sat.ID()]

		user, err := sat.API.DB.Console().Users().GetByEmailAndTenant(ctx, userLogin.Email, nil)
		require.NoError(t, err)

		user.Kind = console.PaidUser
		require.NoError(t, sat.API.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &user.Kind}))

		p, err := sat.AddProject(ctx, user.ID, "test project")
		require.NoError(t, err)

		updateContext := func() (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err := console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}

		userCtx, user := updateContext()

		// set time to middle of day to avoid usage being created in previous month
		// if this test runs early on the first day of the month
		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)

		service.TestSetNow(func() time.Time {
			return timestamp
		})
		sat.API.Payments.StripeService.SetNow(func() time.Time {
			return timestamp
		})
		interval := timestamp.Add(-2 * time.Hour)

		addUsage := func(projectID uuid.UUID, size memory.Size) {
			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))

			interval = interval.Add(time.Hour)

			require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, accounting.BucketStorageTally{
				BucketName:    projectID.String(),
				ProjectID:     projectID,
				IntervalStart: interval,
				TotalBytes:    int64(size),
			}))
		}

		addUsage(p.ID, 400*memory.MB) // large usage that should be more than the minimum fee

		resp, err := service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.CurrentUsage)

		// can't delete bucket storage tallies, so manually delete the project and create another one.
		require.NoError(t, sat.DB.Console().Projects().Delete(ctx, p.ID))
		p2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project 2",
		})
		require.NoError(t, err)

		addUsage(p2.ID, memory.MB) // small usage that should be less than the minimum fee

		resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
		require.NoError(t, err)
		require.Nil(t, resp)

		_, err = sat.DB.ProjectAccounting().ArchiveRollupsBefore(ctx, timestamp, 1)
		require.NoError(t, err)

		// check for usage in previous month, but invoice not generated yet
		// does not affect deletion of free users.
		lastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
		egress := int64(1000000)
		require.NoError(t, sat.DB.Orders().UpdateBucketBandwidthSettle(ctx, p2.ID, p2.ID.Bytes(), pb.PieceAction_GET, egress, 0, lastMonth.Add(time.Hour)))

		resp, err = service.DeleteAccount(userCtx, console.DeleteAccountInit, "test")
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.InvoicingIncomplete)

		thisMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx, []stripe.CreateProjectRecord{{
			ProjectID: p2.ID,
			Egress:    egress,
		}}, lastMonth, thisMonth))

		_, err = service.DeleteAccount(userCtx, console.VerifyAccountPasswordStep, userLogin.Password)
		require.NoError(t, err)
		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountPasswordStep, user.EmailChangeVerificationStep)

		_, err = service.DeleteAccount(userCtx, console.VerifyAccountEmailStep, user.ActivationCode)
		require.NoError(t, err)
		userCtx, user = updateContext()
		require.Equal(t, console.VerifyAccountEmailStep, user.EmailChangeVerificationStep)

		_, err = service.DeleteAccount(userCtx, console.DeleteAccountStep, "test")
		require.NoError(t, err)

		_, user = updateContext()
		require.NoError(t, err)
		require.Equal(t, console.PendingDeletion, user.Status)
	})
}

func TestUpdateUserOnSignup(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.FreeTrialDuration = 48 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		requestData := console.CreateUser{
			FullName:        "old inactive",
			ShortName:       "old inactive",
			Email:           "inactive@mail.test",
			Password:        "old password",
			UserAgent:       []byte("partner1"),
			SignupPromoCode: "test",
			ActivationCode:  "111111",
			SignupId:        "test",
		}

		user, err := service.CreateUser(ctx, requestData, regToken.Secret)
		require.NoError(t, err)
		require.NotNil(t, user)

		requestData.FullName = "new active"
		requestData.ShortName = "new active"
		requestData.Password = "new password"
		requestData.UserAgent = []byte("partnerNew")
		requestData.SignupPromoCode = "new test"
		requestData.ActivationCode = "222222"
		requestData.SignupId = "new test"

		newNow := time.Now().Add(24 * time.Hour)
		service.TestSetNow(func() time.Time {
			return newNow
		})

		err = service.UpdateUserOnSignup(ctx, user, requestData)
		require.NoError(t, err)

		user, err = service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, requestData.FullName, user.FullName)
		require.Equal(t, requestData.ShortName, user.ShortName)
		require.Equal(t, requestData.UserAgent, user.UserAgent)
		require.Equal(t, requestData.SignupPromoCode, user.SignupPromoCode)
		require.Equal(t, requestData.ActivationCode, user.ActivationCode)
		require.Equal(t, requestData.SignupId, user.SignupId)
		require.NotNil(t, user.TrialExpiration)
		require.WithinDuration(t, newNow.Add(sat.Config.Console.FreeTrialDuration), *user.TrialExpiration, time.Minute)

		err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(requestData.Password))
		require.NoError(t, err)
	})
}

func TestPaidTier(t *testing.T) {
	usageConfig := console.UsageLimitsConfig{
		Storage: console.StorageLimitConfig{
			Free: memory.GB,
			Paid: memory.TB,
		},
		Bandwidth: console.BandwidthLimitConfig{
			Free: 2 * memory.GB,
			Paid: 2 * memory.TB,
		},
		Segment: console.SegmentLimitConfig{
			Free: 10,
			Paid: 50,
		},
		Project: console.ProjectLimitConfig{
			Free: 1,
			Paid: 3,
		},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UsageLimits = usageConfig
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		// project should have free tier usage limits
		proj1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Free, *proj1.StorageLimit)
		require.Equal(t, usageConfig.Bandwidth.Free, *proj1.BandwidthLimit)
		require.Equal(t, usageConfig.Segment.Free, *proj1.SegmentLimit)

		// user should be in free tier
		user, err := service.GetUser(ctx, proj1.OwnerID)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, user.Kind)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		// add a credit card to the user
		_, err = service.Payments().AddCreditCard(userCtx, "test-cc-token")
		require.NoError(t, err)

		// expect user to be in paid tier
		user, err = service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, user.Kind)
		require.Equal(t, usageConfig.Project.Paid, user.ProjectLimit)

		// update auth ctx
		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		// expect project to be migrated to paid tier usage limits
		proj1, err = service.GetProject(userCtx, proj1.ID)
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Paid, *proj1.StorageLimit)
		require.Equal(t, usageConfig.Bandwidth.Paid, *proj1.BandwidthLimit)
		require.Equal(t, usageConfig.Segment.Paid, *proj1.SegmentLimit)

		// expect new project to be created with paid tier usage limits
		proj2, err := service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "Project 2"})
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Paid, *proj2.StorageLimit)
	})
}

func TestSetupAccountWithLongNames(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		type requestWithExpectedError struct {
			console.SetUpAccountRequest
			expectErr bool
		}

		ptr := func(s string) *string {
			return &s
		}

		allowedString := string(testrand.RandAlphaNumeric(100))
		disallowedString := string(testrand.RandAlphaNumeric(101))

		tests := []requestWithExpectedError{
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FullName:       ptr("random"),
					IsProfessional: true,
				},
				// first and last names must be provided for professional user.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(disallowedString),
					IsProfessional: true,
				},
				// first name is too long.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(allowedString),
					LastName:       ptr(disallowedString),
					IsProfessional: true,
				},
				// last name is too long.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(allowedString),
					CompanyName:    ptr(disallowedString),
					IsProfessional: true,
				},
				// company name is too long.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(allowedString),
					LastName:       ptr(allowedString),
					IsProfessional: true,
				},
				// company name must be provided.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(allowedString),
					LastName:       ptr(allowedString),
					IsProfessional: false,
				},
				// full name must be provided for non-professional user.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FullName:       ptr(disallowedString),
					IsProfessional: false,
				},
				// full name is too long.
				expectErr: true,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FullName:       ptr(allowedString),
					IsProfessional: false,
				},
				expectErr: false,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(allowedString),
					CompanyName:    ptr(allowedString),
					IsProfessional: true,
				},
				// last name is not required.
				expectErr: false,
			},
			{
				SetUpAccountRequest: console.SetUpAccountRequest{
					FirstName:      ptr(allowedString),
					LastName:       ptr(allowedString),
					CompanyName:    ptr(allowedString),
					IsProfessional: true,
				},
				expectErr: false,
			},
		}

		for _, tt := range tests {
			err = service.SetupAccount(userCtx, tt.SetUpAccountRequest)
			if tt.expectErr {
				require.True(t, console.ErrValidation.Has(err))
			} else {
				require.NoError(t, err)
			}
		}
	})
}

// TestUpdateProjectExceedsLimits ensures that a project with limits manually set above the defaults can be updated.
func TestUpdateProjectExceedsLimits(t *testing.T) {
	usageConfig := console.UsageLimitsConfig{
		Storage: console.StorageLimitConfig{
			Free: memory.GB,
			Paid: memory.TB,
		},
		Bandwidth: console.BandwidthLimitConfig{
			Free: 2 * memory.GB,
			Paid: 2 * memory.TB,
		},
		Segment: console.SegmentLimitConfig{
			Free: 10,
			Paid: 50,
		},
		Project: console.ProjectLimitConfig{
			Free: 1,
			Paid: 3,
		},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UsageLimits = usageConfig
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		projectID := planet.Uplinks[0].Projects[0].ID

		updatedName := "newName"
		updatedDescription := "newDescription"
		updatedStorageLimit := memory.Size(100) + memory.TB
		updatedBandwidthLimit := memory.Size(100) + memory.TB

		proj, err := sat.API.DB.Console().Projects().Get(ctx, projectID)
		require.NoError(t, err)

		userCtx1, err := sat.UserContext(ctx, proj.OwnerID)
		require.NoError(t, err)

		// project should have free tier usage limits
		require.Equal(t, usageConfig.Storage.Free, *proj.StorageLimit)
		require.Equal(t, usageConfig.Bandwidth.Free, *proj.BandwidthLimit)
		require.Equal(t, usageConfig.Segment.Free, *proj.SegmentLimit)

		// update project name should succeed
		_, err = service.UpdateProject(userCtx1, projectID, console.UpsertProjectInfo{
			Name:        updatedName,
			Description: updatedDescription,
		})
		require.NoError(t, err)

		// manually set project limits above defaults
		proj1, err := sat.API.DB.Console().Projects().Get(ctx, projectID)
		require.NoError(t, err)
		proj1.StorageLimit = new(memory.Size)
		*proj1.StorageLimit = updatedStorageLimit
		proj1.BandwidthLimit = new(memory.Size)
		*proj1.BandwidthLimit = updatedBandwidthLimit
		err = sat.DB.Console().Projects().Update(ctx, proj1)
		require.NoError(t, err)

		// try to update project name should succeed
		_, err = service.UpdateProject(userCtx1, projectID, console.UpsertProjectInfo{
			Name:        "updatedName",
			Description: "updatedDescription",
		})
		require.NoError(t, err)
	})
}

func TestMFA(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "MFA Test User",
			Email:    "mfauser@mail.test",
		}, 1)
		require.NoError(t, err)

		updateContext := func() (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err := console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}
		userCtx, user := updateContext()

		mfaTime := time.Now()
		var key string
		t.Run("ResetMFASecretKey", func(t *testing.T) {
			key, err = service.ResetMFASecretKey(userCtx)
			require.NoError(t, err)

			_, user := updateContext()
			require.NotEmpty(t, user.MFASecretKey)
		})

		t.Run("EnableUserMFABadPasscode", func(t *testing.T) {
			// Expect MFA-enabling attempt to be rejected when providing stale passcode.
			badCode, err := console.NewMFAPasscode(key, mfaTime.Add(time.Hour))
			require.NoError(t, err)

			err = service.EnableUserMFA(userCtx, badCode, mfaTime)
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext()
			_, err = service.ResetMFARecoveryCodes(userCtx, false, "", "")
			require.True(t, console.ErrUnauthorized.Has(err))

			_, user = updateContext()
			require.False(t, user.MFAEnabled)
		})

		t.Run("EnableUserMFAGoodPasscode", func(t *testing.T) {
			// Expect MFA-enabling attempt to succeed when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, mfaTime)
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.EnableUserMFA(userCtx, goodCode, mfaTime)
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.Equal(t, user.MFASecretKey, key)

			err = service.EnableUserMFA(userCtx, goodCode, mfaTime)
			require.True(t, console.ErrMFAEnabled.Has(err))
		})

		t.Run("MFAGetToken", func(t *testing.T) {
			request := console.AuthUser{Email: user.Email, Password: user.FullName}

			// Expect no token due to lack of MFA passcode.
			token, err := service.Token(ctx, request)
			require.True(t, console.ErrMFAMissing.Has(err))
			require.Empty(t, token)

			// Expect no token due to bad MFA passcode.
			wrongCode, err := console.NewMFAPasscode(key, time.Now().Add(time.Hour))
			require.NoError(t, err)

			request.MFAPasscode = wrongCode
			token, err = service.Token(ctx, request)
			require.True(t, console.ErrMFAPasscode.Has(err))
			require.Empty(t, token)

			// Expect token when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Now())
			require.NoError(t, err)

			request.MFAPasscode = goodCode
			token, err = service.Token(ctx, request)
			require.NoError(t, err)
			require.NotEmpty(t, token)
		})

		t.Run("MFARecoveryCodes", func(t *testing.T) {
			_, err = service.ResetMFARecoveryCodes(userCtx, false, "", "")
			require.NoError(t, err)

			_, user = updateContext()
			require.Len(t, user.MFARecoveryCodes, console.MFARecoveryCodeCount)

			for _, code := range user.MFARecoveryCodes {
				// Ensure code is of the form XXXX-XXXX-XXXX where X is A-Z or 0-9.
				require.Regexp(t, "^([A-Z0-9]{4})((-[A-Z0-9]{4})){2}$", code)

				// Expect token when providing valid recovery code.
				request := console.AuthUser{Email: user.Email, Password: user.FullName, MFARecoveryCode: code}
				token, err := service.Token(ctx, request)
				require.NoError(t, err)
				require.NotEmpty(t, token)

				// Expect no token due to providing previously-used recovery code.
				token, err = service.Token(ctx, request)
				require.True(t, console.ErrMFARecoveryCode.Has(err))
				require.Empty(t, token)

				_, user = updateContext()
			}

			userCtx, _ = updateContext()

			// requiring MFA code to reset recovery codes should work
			code, err := console.NewMFAPasscode(key, mfaTime)
			require.NoError(t, err)
			_, err = service.ResetMFARecoveryCodes(userCtx, true, code, "")
			require.NoError(t, err)
		})

		t.Run("DisableUserMFABadPasscode", func(t *testing.T) {
			// Expect MFA-disabling attempt to fail when providing valid passcode.
			badCode, err := console.NewMFAPasscode(key, mfaTime.Add(time.Hour))
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.DisableUserMFA(userCtx, badCode, mfaTime, "")
			require.True(t, console.ErrValidation.Has(err))

			_, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.NotEmpty(t, user.MFASecretKey)
			require.NotEmpty(t, user.MFARecoveryCodes)
		})

		t.Run("DisableUserMFAConflict", func(t *testing.T) {
			// Expect MFA-disabling attempt to fail when providing both recovery code and passcode.
			goodCode, err := console.NewMFAPasscode(key, mfaTime)
			require.NoError(t, err)

			userCtx, user = updateContext()
			err = service.DisableUserMFA(userCtx, goodCode, mfaTime, user.MFARecoveryCodes[0])
			require.True(t, console.ErrMFAConflict.Has(err))

			_, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.NotEmpty(t, user.MFASecretKey)
			require.NotEmpty(t, user.MFARecoveryCodes)
		})

		t.Run("DisableUserMFAGoodPasscode", func(t *testing.T) {
			// Expect MFA-disabling attempt to succeed when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, mfaTime)
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.DisableUserMFA(userCtx, goodCode, mfaTime, "")
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.False(t, user.MFAEnabled)
			require.Empty(t, user.MFASecretKey)
			require.Empty(t, user.MFARecoveryCodes)
		})

		t.Run("DisableUserMFAGoodRecoveryCode", func(t *testing.T) {
			// Expect MFA-disabling attempt to succeed when providing valid recovery code.
			// Enable MFA
			key, err = service.ResetMFASecretKey(userCtx)
			require.NoError(t, err)

			goodCode, err := console.NewMFAPasscode(key, mfaTime)
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.EnableUserMFA(userCtx, goodCode, mfaTime)
			require.NoError(t, err)

			userCtx, _ = updateContext()
			_, err = service.ResetMFARecoveryCodes(userCtx, false, "", "")
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.NotEmpty(t, user.MFASecretKey)
			require.NotEmpty(t, user.MFARecoveryCodes)

			// Disable MFA
			err = service.DisableUserMFA(userCtx, "", mfaTime, user.MFARecoveryCodes[0])
			require.NoError(t, err)

			_, user = updateContext()
			require.False(t, user.MFAEnabled)
			require.Empty(t, user.MFASecretKey)
			require.Empty(t, user.MFARecoveryCodes)
		})
	})
}

func TestResetPassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		newPass := user.FullName

		getNewResetToken := func() *console.ResetPasswordToken {
			token, err := sat.DB.Console().ResetPasswordTokens().Create(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, token)
			return token
		}
		token := getNewResetToken()

		// Expect error when providing bad token.
		err = service.ResetPassword(ctx, "badToken", newPass, "", "", token.CreatedAt)
		require.True(t, console.ErrRecoveryToken.Has(err))

		// Expect error when providing good but expired token.
		err = service.ResetPassword(
			ctx,
			token.Secret.String(),
			newPass,
			"",
			"",
			token.CreatedAt.Add(sat.Config.ConsoleAuth.TokenExpirationTime).Add(time.Second),
		)
		require.True(t, console.ErrTokenExpiration.Has(err))

		// Expect error when providing good token with bad (too short) password.
		err = service.ResetPassword(ctx, token.Secret.String(), "bad", "", "", token.CreatedAt)
		require.True(t, console.ErrValidation.Has(err))

		// Expect success when providing good token and good password.
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, "", "", token.CreatedAt)
		require.NoError(t, err)

		token = getNewResetToken()

		// Enable MFA.
		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		key, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)
		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		passcode, err := console.NewMFAPasscode(key, token.CreatedAt)
		require.NoError(t, err)

		err = service.EnableUserMFA(userCtx, passcode, token.CreatedAt)
		require.NoError(t, err)

		// Expect error when providing bad passcode.
		badPasscode, err := console.NewMFAPasscode(key, token.CreatedAt.Add(time.Hour))
		require.NoError(t, err)
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, "", token.CreatedAt)
		require.True(t, console.ErrMFAPasscode.Has(err))

		for _, recoveryCode := range user.MFARecoveryCodes {
			// Expect success when providing bad passcode and good recovery code.
			err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, recoveryCode, token.CreatedAt)
			require.NoError(t, err)
			token = getNewResetToken()

			// Expect error when providing bad passcode and already-used recovery code.
			err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, recoveryCode, token.CreatedAt)
			require.True(t, console.ErrMFARecoveryCode.Has(err))
		}

		// Expect success when providing good passcode.
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, passcode, "", token.CreatedAt)
		require.NoError(t, err)

		// Expect account to be locked when providing bad passcode or recovery code 3 times in a row.
		token = getNewResetToken()
		require.NotNil(t, token)
		badPasscode, err = console.NewMFAPasscode(key, token.CreatedAt.Add(time.Hour))
		require.NoError(t, err)

		for i := 0; i < sat.Config.Console.LoginAttemptsWithoutPenalty; i++ {
			err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, "", token.CreatedAt)
			require.True(t, console.ErrMFAPasscode.Has(err))
		}

		err = service.ResetPassword(ctx, token.Secret.String(), newPass, badPasscode, "", token.CreatedAt)
		require.True(t, console.ErrTooManyAttempts.Has(err))

		err = service.ResetAccountLock(ctx, user)
		require.NoError(t, err)

		badRecoveryCode := "badRecovery"
		for i := 0; i < sat.Config.Console.LoginAttemptsWithoutPenalty; i++ {
			err = service.ResetPassword(ctx, token.Secret.String(), newPass, "", badRecoveryCode, token.CreatedAt)
			require.True(t, console.ErrMFARecoveryCode.Has(err))
		}

		err = service.ResetPassword(ctx, token.Secret.String(), newPass, "", badRecoveryCode, token.CreatedAt)
		require.True(t, console.ErrTooManyAttempts.Has(err))

		err = service.ResetAccountLock(ctx, user)
		require.NoError(t, err)

		// Expect success when providing good passcode.
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, passcode, "", token.CreatedAt)
		require.NoError(t, err)
	})
}

func TestChangePassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		newPass := "newPass123!"

		user, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, upl.User[sat.ID()].Email, nil)
		require.NoError(t, err)
		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		for i := 0; i < 2; i++ {
			_, err = sat.API.Console.Service.GenerateSessionToken(userCtx, console.SessionTokenRequest{
				UserID:          user.ID,
				Email:           user.Email,
				IP:              "",
				UserAgent:       "",
				AnonymousID:     "",
				CustomDuration:  nil,
				HubspotObjectID: user.HubspotObjectID,
			})
			require.NoError(t, err)
		}
		sessions, err := sat.DB.Console().WebappSessions().GetAllByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, sessions, 2)

		// generate a password recovery token to test that changing password invalidates it
		passwordRecoveryToken, err := sat.API.Console.Service.GeneratePasswordRecoveryToken(userCtx, user)
		require.NoError(t, err)

		sessionID := sessions[0].ID
		require.NoError(t, sat.API.Console.Service.ChangePassword(userCtx, upl.User[sat.ID()].Password, newPass, &sessionID))
		user, err = sat.DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.NoError(t, bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(newPass)))

		// change password should've deleted other sessions.
		sessions, err = sat.DB.Console().WebappSessions().GetAllByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, sessions, 1)
		require.Equal(t, sessionID, sessions[0].ID)

		err = sat.API.Console.Service.ResetPassword(
			userCtx,
			passwordRecoveryToken,
			"aDifferentPassword123!",
			"",
			"",
			time.Now(),
		)
		require.Error(t, err)
		require.True(t, console.ErrRecoveryToken.Has(err))
	})
}

func TestGenerateSessionToken(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Session.InactivityTimerEnabled = true
				config.Console.Session.InactivityTimerDuration = 600
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service

		user, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		now := time.Now()
		token1, err := srv.GenerateSessionToken(userCtx, console.SessionTokenRequest{
			UserID:          user.ID,
			Email:           user.Email,
			IP:              "",
			UserAgent:       "",
			AnonymousID:     "",
			CustomDuration:  nil,
			HubspotObjectID: user.HubspotObjectID,
		})
		require.NoError(t, err)
		require.NotNil(t, token1)

		token1Duration := token1.ExpiresAt.Sub(now)
		increase := 10 * time.Minute
		increasedDuration := time.Duration(sat.Config.Console.Session.InactivityTimerDuration)*time.Second + increase
		ptr := &increasedDuration
		require.NoError(t, sat.DB.Console().Users().UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
			SessionDuration: &ptr,
		}))

		now = time.Now()
		token2, err := srv.GenerateSessionToken(userCtx, console.SessionTokenRequest{
			UserID:          user.ID,
			Email:           user.Email,
			IP:              "",
			UserAgent:       "",
			AnonymousID:     "",
			CustomDuration:  nil,
			HubspotObjectID: user.HubspotObjectID,
		})
		require.NoError(t, err)
		token2Duration := token2.ExpiresAt.Sub(now)
		require.Greater(t, token2Duration, token1Duration)

		decrease := -5 * time.Minute
		decreasedDuration := time.Duration(sat.Config.Console.Session.InactivityTimerDuration)*time.Second + decrease
		ptr = &decreasedDuration
		require.NoError(t, sat.DB.Console().Users().UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
			SessionDuration: &ptr,
		}))

		now = time.Now()
		token3, err := srv.GenerateSessionToken(userCtx, console.SessionTokenRequest{
			UserID:          user.ID,
			Email:           user.Email,
			IP:              "",
			UserAgent:       "",
			AnonymousID:     "",
			CustomDuration:  nil,
			HubspotObjectID: user.HubspotObjectID,
		})
		require.NoError(t, err)
		token3Duration := token3.ExpiresAt.Sub(now)
		require.Less(t, token3Duration, token1Duration)

		now = time.Now()
		customDuration := 7 * 24 * time.Hour
		inAWeek := now.Add(customDuration)
		token4, err := srv.GenerateSessionToken(userCtx, console.SessionTokenRequest{
			UserID:          user.ID,
			Email:           user.Email,
			IP:              "",
			UserAgent:       "",
			AnonymousID:     "",
			CustomDuration:  &customDuration,
			HubspotObjectID: user.HubspotObjectID,
		})
		require.NoError(t, err)
		require.True(t, token4.ExpiresAt.After(inAWeek))
	})
}

func TestRefreshSessionToken(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Session.InactivityTimerEnabled = true
				config.Console.Session.InactivityTimerDuration = 600
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service

		user, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		now := time.Now()
		token, err := srv.GenerateSessionToken(userCtx, console.SessionTokenRequest{
			UserID:          user.ID,
			Email:           user.Email,
			IP:              "",
			UserAgent:       "",
			AnonymousID:     "",
			CustomDuration:  nil,
			HubspotObjectID: user.HubspotObjectID,
		})
		require.NoError(t, err)
		require.NotNil(t, token)

		defaultDuration := token.ExpiresAt.Sub(now)
		increase := 10 * time.Minute
		increasedDuration := time.Duration(sat.Config.Console.Session.InactivityTimerDuration)*time.Second + increase
		ptr := &increasedDuration
		require.NoError(t, sat.DB.Console().Users().UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
			SessionDuration: &ptr,
		}))

		sessionID, err := uuid.FromBytes(token.Token.Payload)
		require.NoError(t, err)

		now = time.Now()
		increasedExpiration, err := srv.RefreshSession(userCtx, sessionID)
		require.NoError(t, err)
		require.Greater(t, increasedExpiration.Sub(now), defaultDuration)

		decrease := -5 * time.Minute
		decreasedDuration := time.Duration(sat.Config.Console.Session.InactivityTimerDuration)*time.Second + decrease
		ptr = &decreasedDuration
		require.NoError(t, sat.DB.Console().Users().UpsertSettings(ctx, user.ID, console.UpsertUserSettingsRequest{
			SessionDuration: &ptr,
		}))

		now = time.Now()
		decreasedExpiration, err := srv.RefreshSession(userCtx, sessionID)
		require.NoError(t, err)
		require.Less(t, decreasedExpiration.Sub(now), defaultDuration)
	})
}

func TestLoginRestricted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		userDB := sat.DB.Console().Users()
		user := planet.Uplinks[0].User[sat.ID()]

		dbUser, _, err := service.GetUserByEmailWithUnverified(ctx, user.Email)
		require.NoError(t, err)

		status := console.PendingBotVerification
		err = userDB.Update(ctx, dbUser.ID, console.UpdateUserRequest{Status: &status})
		require.NoError(t, err)

		tokenInfo, err := service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.Password})
		require.True(t, console.ErrLoginRestricted.Has(err))
		require.Nil(t, tokenInfo)

		status = console.LegalHold
		err = userDB.Update(ctx, dbUser.ID, console.UpdateUserRequest{Status: &status})
		require.NoError(t, err)

		tokenInfo, err = service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.Password})
		require.True(t, console.ErrLoginRestricted.Has(err))
		require.Nil(t, tokenInfo)

		status = console.Active
		err = userDB.Update(ctx, dbUser.ID, console.UpdateUserRequest{Status: &status})
		require.NoError(t, err)

		tokenInfo, err = service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.Password})
		require.NoError(t, err)
		require.NotNil(t, tokenInfo)
	})
}

func TestUserSettings(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service
		userDB := sat.DB.Console().Users()

		existingUser, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, existingUser.ID)
		require.NoError(t, err)

		// getting non-existing settings directly from db should return error
		_, err = userDB.GetSettings(userCtx, existingUser.ID)
		require.Error(t, err)

		// a user that already has a project prior to getting user settings should not go through onboarding again
		// in other words, onboarding start and end should both be true for users who have a project
		settings, err := srv.GetUserSettings(userCtx)
		require.NoError(t, err)
		require.Equal(t, true, settings.OnboardingStart)
		require.Equal(t, true, settings.OnboardingEnd)
		require.Nil(t, settings.OnboardingStep)
		require.Nil(t, settings.SessionDuration)

		noticeDismissal := console.NoticeDismissal{
			FileGuide:                false,
			ServerSideEncryption:     false,
			PartnerUpgradeBanner:     false,
			ProjectMembersPassphrase: false,
			UploadOverwriteWarning:   false,
		}
		require.Equal(t, noticeDismissal, settings.NoticeDismissal)

		newUser, err := userDB.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "newuser@example.test",
			PasswordHash: []byte("i am a hash of a password, hello"),
		})
		require.NoError(t, err)

		userCtx, err = sat.UserContext(ctx, newUser.ID)
		require.NoError(t, err)

		// a brand new user with no project should go through onboarding
		// in other words, onboarding start and end should both be false for users withouut a project
		settings, err = srv.GetUserSettings(userCtx)
		require.NoError(t, err)
		require.Equal(t, false, settings.OnboardingStart)
		require.Equal(t, false, settings.OnboardingEnd)
		require.Nil(t, settings.OnboardingStep)
		require.Nil(t, settings.SessionDuration)
		require.Equal(t, noticeDismissal, settings.NoticeDismissal)

		onboardingBool := true
		onboardingStep := "Overview"
		sessionDur := time.Duration(rand.Int63()).Round(time.Minute)
		sessionDurPtr := &sessionDur
		noticeDismissal.ServerSideEncryption = true
		noticeDismissal.FileGuide = true
		noticeDismissal.PartnerUpgradeBanner = true
		noticeDismissal.ProjectMembersPassphrase = true
		noticeDismissal.UploadOverwriteWarning = true
		settings, err = srv.SetUserSettings(userCtx, console.UpsertUserSettingsRequest{
			SessionDuration: &sessionDurPtr,
			OnboardingStart: &onboardingBool,
			OnboardingEnd:   &onboardingBool,
			OnboardingStep:  &onboardingStep,
			NoticeDismissal: &noticeDismissal,
		})
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)
		require.Equal(t, noticeDismissal, settings.NoticeDismissal)

		settings, err = userDB.GetSettings(userCtx, newUser.ID)
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)
		require.Equal(t, noticeDismissal, settings.NoticeDismissal)

		// passing nil should not override existing values
		settings, err = srv.SetUserSettings(userCtx, console.UpsertUserSettingsRequest{
			SessionDuration: nil,
			OnboardingStart: nil,
			OnboardingEnd:   nil,
			OnboardingStep:  nil,
			NoticeDismissal: nil,
		})
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)
		require.Equal(t, noticeDismissal, settings.NoticeDismissal)

		settings, err = userDB.GetSettings(userCtx, newUser.ID)
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)
		require.Equal(t, noticeDismissal, settings.NoticeDismissal)
	})
}

func TestSetActivationCodeAndSignupID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service

		existingUser, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.Empty(t, existingUser.ActivationCode)

		// should not work with active status
		require.Equal(t, console.Active, existingUser.Status)
		updatedUser, err := srv.SetActivationCodeAndSignupID(ctx, *existingUser)
		require.Error(t, err)
		require.Equal(t, console.User{}, updatedUser)

		// should work with inactive status
		newStatus := console.Inactive
		err = sat.DB.Console().Users().Update(ctx, existingUser.ID, console.UpdateUserRequest{
			Status: &newStatus,
		})
		require.NoError(t, err)

		activeUser, inactiveUsers, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.Nil(t, activeUser)
		require.Len(t, inactiveUsers, 1)
		existingUser = &inactiveUsers[0]

		require.Empty(t, existingUser.ActivationCode)
		updatedUser2, err := srv.SetActivationCodeAndSignupID(ctx, *existingUser)
		require.NoError(t, err)
		require.NotEmpty(t, updatedUser2.ActivationCode)

		// should be possible to get new activation code
		updatedUser3, err := srv.SetActivationCodeAndSignupID(ctx, *existingUser)
		require.NoError(t, err)
		require.NotEqual(t, updatedUser2.ActivationCode, updatedUser3.ActivationCode)

		// should not work with a status that is not "Inactive"
		newStatus = console.PendingDeletion
		err = sat.DB.Console().Users().Update(ctx, existingUser.ID, console.UpdateUserRequest{
			Status: &newStatus,
		})
		require.NoError(t, err)
		activeUser, inactiveUsers, err = srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)
		require.Nil(t, activeUser)
		require.Len(t, inactiveUsers, 1)
		existingUser = &inactiveUsers[0]

		updatedUser4, err := srv.SetActivationCodeAndSignupID(ctx, *existingUser)
		require.Error(t, err)
		require.Equal(t, console.User{}, updatedUser4)
	})
}

func TestRESTKeys(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, runRestKeysTest)
}

func TestRESTKeys_WithNewTable(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.UseNewRestKeysTable = true
			},
		},
	}, runRestKeysTest)
}

func runRestKeysTest(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	sat := planet.Satellites[0]
	consoleService := sat.API.Console.Service
	service := sat.API.Console.RestKeys

	proj1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
	require.NoError(t, err)

	user, err := consoleService.GetUser(ctx, proj1.OwnerID)
	require.NoError(t, err)

	now := time.Now()
	err = sat.API.DB.Console().Users().UpdatePaidTier(ctx, user.ID, true, memory.PB, memory.PB, 1000000, 3, &now)
	require.NoError(t, err)

	userCtx, err := sat.UserContext(ctx, user.ID)
	require.NoError(t, err)

	expires := 5 * time.Hour
	apiKey, expiresAt, err := service.Create(userCtx, "api key", &expires)
	if sat.Config.Console.UseNewRestKeysTable {
		require.NoError(t, err)
		require.NotEmpty(t, apiKey)
		require.True(t, expiresAt.After(now))
		require.True(t, expiresAt.Before(now.Add(expires+time.Hour)))
	} else {
		// using Create when the new table is not used will return an error
		require.Error(t, err)
		apiKey, expiresAt, err = service.CreateNoAuth(ctx, user.ID, &expires)
		require.NoError(t, err)
		require.NotEmpty(t, apiKey)
		require.True(t, expiresAt.After(now))
		require.True(t, expiresAt.Before(now.Add(expires+time.Hour)))
	}

	// test GetUserFromKey
	userID, exp, err := service.GetUserAndExpirationFromKey(ctx, apiKey)
	require.NoError(t, err)
	require.Equal(t, user.ID, userID)
	require.False(t, exp.IsZero())
	require.False(t, exp.Before(now))

	// test revocation
	require.NoError(t, service.RevokeByKeyNoAuth(userCtx, apiKey))

	// test revoke non existent key
	nonexistent := testrand.UUID()
	err = service.RevokeByKeyNoAuth(userCtx, nonexistent.String())
	require.Error(t, err)

	if !sat.Config.Console.UseNewRestKeysTable {
		// GetAll should return an error when the new table is not used
		_, err = service.GetAll(userCtx)
		require.Error(t, err)

		err = service.RevokeByIDs(userCtx, []uuid.UUID{})
		require.Error(t, err)
		return
	}

	keys, err := service.GetAll(userCtx)
	require.NoError(t, err)
	require.Empty(t, keys)

	expires = 50 * time.Hour
	_, _, err = service.Create(userCtx, "key1", &expires)
	require.NoError(t, err)
	_, _, err = service.Create(userCtx, "key2", &expires)
	require.NoError(t, err)

	keys, err = service.GetAll(userCtx)
	require.NoError(t, err)
	require.Len(t, keys, 2)

	err = service.RevokeByIDs(userCtx, []uuid.UUID{keys[0].ID})
	require.NoError(t, err)

	keys, err = service.GetAll(userCtx)
	require.NoError(t, err)
	require.Len(t, keys, 1)

	// test GetUserFromKey non-existent key
	_, _, err = service.GetUserAndExpirationFromKey(ctx, testrand.UUID().String())
	require.True(t, console.ErrInvalidKey.Has(err))
}

func TestRESTKeysExpiration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, runRestKeysExpirationTest)
}

func TestRESTKeysExpiration_WithNewTable(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, runRestKeysExpirationTest)
}

func runRestKeysExpirationTest(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	sat := planet.Satellites[0]
	service := sat.API.Console.RestKeys

	user, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].User[sat.ID()].Email, nil)
	require.NoError(t, err)

	now := time.Now()

	dur := time.Duration(0)
	// test no expiration uses default
	_, expiresAt, err := service.CreateNoAuth(ctx, user.ID, &dur)
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(sat.Config.Console.RestAPIKeys.DefaultExpiration), *expiresAt, time.Minute)

	_, expiresAt, err = service.CreateNoAuth(ctx, user.ID, nil)
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(sat.Config.Console.RestAPIKeys.DefaultExpiration), *expiresAt, time.Minute)

	// test negative expiration uses default
	dur = -1
	_, expiresAt, err = service.CreateNoAuth(ctx, user.ID, &dur)
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(sat.Config.Console.RestAPIKeys.DefaultExpiration), *expiresAt, time.Minute)

	// test regular expiration
	dur = 14 * time.Hour
	_, expiresAt, err = service.CreateNoAuth(ctx, user.ID, &dur)
	require.NoError(t, err)
	require.WithinDuration(t, now.Add(dur), *expiresAt, time.Minute)
}

// TestLockAccount ensures user's gets locked when incorrect credentials are provided.
func TestLockAccount(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()
		consoleConfig := sat.Config.Console

		newUser := console.CreateUser{
			FullName: "token test",
			Email:    "token_test@mail.test",
		}

		user, err := sat.AddUser(ctx, newUser, 1)
		require.NoError(t, err)

		updateContext := func() (context.Context, *console.User) {
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			user, err := console.GetUser(userCtx)
			require.NoError(t, err)
			return userCtx, user
		}

		userCtx, _ := updateContext()
		secret, err := service.ResetMFASecretKey(userCtx)
		require.NoError(t, err)

		goodCode0, err := console.NewMFAPasscode(secret, time.Time{})
		require.NoError(t, err)

		userCtx, _ = updateContext()
		err = service.EnableUserMFA(userCtx, goodCode0, time.Time{})
		require.NoError(t, err)

		now := time.Now()

		goodCode1, err := console.NewMFAPasscode(secret, now)
		require.NoError(t, err)

		authUser := console.AuthUser{
			Email:       newUser.Email,
			Password:    newUser.FullName,
			MFAPasscode: goodCode1,
		}

		// successful login.
		token, err := service.Token(ctx, authUser)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// check if user's account gets locked because of providing wrong password.
		authUser.Password = "qweQWE1@"
		for i := 1; i <= consoleConfig.LoginAttemptsWithoutPenalty; i++ {
			token, err = service.Token(ctx, authUser)
			require.Empty(t, token)
			require.True(t, console.ErrLoginCredentials.Has(err))
		}

		lockedUser, err := service.GetUser(userCtx, user.ID)
		require.NoError(t, err)
		require.True(t, lockedUser.FailedLoginCount == consoleConfig.LoginAttemptsWithoutPenalty)
		require.True(t, lockedUser.LoginLockoutExpiration.After(now))

		// lock account once again and check if lockout expiration time increased.
		_, err = service.UpdateUsersFailedLoginState(userCtx, lockedUser)
		require.NoError(t, err)

		lockedUser, err = service.GetUser(userCtx, user.ID)
		require.NoError(t, err)
		require.True(t, lockedUser.FailedLoginCount == consoleConfig.LoginAttemptsWithoutPenalty+1)

		diff := lockedUser.LoginLockoutExpiration.Sub(now)
		require.Greater(t, diff, time.Duration(consoleConfig.FailedLoginPenalty)*time.Minute)

		// unlock account by successful login
		lockedUser.LoginLockoutExpiration = now.Add(-time.Second)
		lockoutExpirationPtr := &lockedUser.LoginLockoutExpiration
		err = usersDB.Update(userCtx, lockedUser.ID, console.UpdateUserRequest{
			LoginLockoutExpiration: &lockoutExpirationPtr,
		})
		require.NoError(t, err)

		authUser.Password = newUser.FullName
		token, err = service.Token(ctx, authUser)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		unlockedUser, err := service.GetUser(userCtx, user.ID)
		require.NoError(t, err)
		require.Zero(t, unlockedUser.FailedLoginCount)

		// check if user's account gets locked because of providing wrong mfa passcode.
		authUser.MFAPasscode = "000000"
		for i := 1; i <= consoleConfig.LoginAttemptsWithoutPenalty; i++ {
			token, err = service.Token(ctx, authUser)
			require.Empty(t, token)
			require.True(t, console.ErrMFAPasscode.Has(err))
		}

		lockedUser, err = service.GetUser(userCtx, user.ID)
		require.NoError(t, err)
		require.True(t, lockedUser.FailedLoginCount == consoleConfig.LoginAttemptsWithoutPenalty)
		require.True(t, lockedUser.LoginLockoutExpiration.After(now))

		// unlock account
		lockedUser.LoginLockoutExpiration = time.Time{}
		lockoutExpirationPtr = &lockedUser.LoginLockoutExpiration
		lockedUser.FailedLoginCount = 0
		err = usersDB.Update(userCtx, lockedUser.ID, console.UpdateUserRequest{
			LoginLockoutExpiration: &lockoutExpirationPtr,
			FailedLoginCount:       &lockedUser.FailedLoginCount,
		})
		require.NoError(t, err)

		// check if user's account gets locked because of providing wrong mfa recovery code.
		authUser.MFAPasscode = ""
		authUser.MFARecoveryCode = "000000"
		for i := 1; i <= consoleConfig.LoginAttemptsWithoutPenalty; i++ {
			token, err = service.Token(ctx, authUser)
			require.Empty(t, token)
			require.True(t, console.ErrMFARecoveryCode.Has(err))
		}

		lockedUser, err = service.GetUser(userCtx, user.ID)
		require.NoError(t, err)
		require.True(t, lockedUser.FailedLoginCount == consoleConfig.LoginAttemptsWithoutPenalty)
		require.True(t, lockedUser.LoginLockoutExpiration.After(now))
	})
}

func TestWalletJsonMarshall(t *testing.T) {
	wi := console.WalletInfo{
		Address: blockchain.Address{1, 2, 3},
		Balance: currency.AmountFromBaseUnits(10000, currency.USDollars),
	}

	out, err := json.Marshal(wi)
	require.NoError(t, err)
	require.Contains(t, string(out), "\"address\":\"0x0102030000000000000000000000000000000000\"")
	require.Contains(t, string(out), "\"balance\":{\"value\":\"100\",\"currency\":\"USD\"}")
}

func TestSessionExpiration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Session.InactivityTimerEnabled = false
				config.Console.Session.Duration = time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		// Session should be added to DB after token request
		tokenInfo, err := service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		_, _, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		sessionID, err := uuid.FromBytes(tokenInfo.Token.Payload)
		require.NoError(t, err)

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID)
		require.NoError(t, err)

		// Session should be removed from DB after it has expired
		_, _, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now().Add(2*time.Hour))
		require.True(t, console.ErrTokenExpiration.Has(err))

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID)
		require.ErrorIs(t, sql.ErrNoRows, err)
	})
}

func TestTrialExpirationImmediate(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.FreeTrialDuration = 0
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)
		require.Nil(t, user.TrialExpiration)
	})
}

func TestTrialExpirationDelayed(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.FreeTrialDuration = 48 * time.Hour
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		now := time.Now()
		service.TestSetNow(func() time.Time {
			return now
		})

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)
		require.NotNil(t, user.TrialExpiration)
		require.WithinDuration(t, now.Add(sat.Config.Console.FreeTrialDuration), *user.TrialExpiration, time.Minute)
	})
}

func TestDeleteAllSessionsByUserIDExcept(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		// Session should be added to DB after token request
		tokenInfo, err := service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		_, _, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		sessionID, err := uuid.FromBytes(tokenInfo.Token.Payload)
		require.NoError(t, err)

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID)
		require.NoError(t, err)

		// Session2 should be added to DB after token request
		tokenInfo2, err := service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		_, _, err = service.TokenAuth(ctx, tokenInfo2.Token, time.Now())
		require.NoError(t, err)

		sessionID2, err := uuid.FromBytes(tokenInfo2.Token.Payload)
		require.NoError(t, err)

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID2)
		require.NoError(t, err)

		// Session2 should be removed from DB after calling DeleteAllSessionByUserIDExcept with Session1
		err = service.DeleteAllSessionsByUserIDExcept(ctx, user.ID, sessionID)
		require.NoError(t, err)

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID2)
		require.ErrorIs(t, sql.ErrNoRows, err)
	})
}

func TestSatelliteManagedProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,

		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SatelliteManagedEncryptionEnabled = true
				config.Console.ManagedEncryption.PathEncryptionEnabled = true
				config.KeyManagement.KeyInfos = kms.KeyInfos{
					Values: map[int]kms.KeyInfo{
						1: {
							SecretVersion: "secretversion1", SecretChecksum: 12345,
						},
						2: {
							SecretVersion: "secretversion2", SecretChecksum: 54321,
						},
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service
		kmsService := sat.API.KeyManagement.Service
		projectDB := sat.DB.Console().Projects()

		existingUser, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, existingUser.ID)
		require.NoError(t, err)

		project, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project",
			ManagePassphrase: false,
		})
		require.NoError(t, err)
		require.NotNil(t, project.PathEncryption)
		require.True(t, *project.PathEncryption)

		p1EncPass, p1EncKeyID, err := projectDB.GetEncryptedPassphrase(userCtx, project.ID)
		require.NoError(t, err)
		// encryptedPassphrase should be empty because project encryption is not managed by satellite
		require.Empty(t, p1EncPass)
		require.Nil(t, p1EncKeyID)

		config, err := srv.GetProjectConfig(userCtx, project.ID)
		require.NoError(t, err)
		require.Empty(t, config.Passphrase)

		pathEncProj, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Path Enc Project",
			ManagePassphrase: true,
		})
		require.NoError(t, err)
		require.NotNil(t, pathEncProj.PathEncryption)
		require.True(t, *pathEncProj.PathEncryption)

		srv.TestToggleManagedEncryptionPathEncryption(false)

		project2, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project2",
			ManagePassphrase: true,
		})
		require.NoError(t, err)
		require.NotNil(t, project2.PathEncryption)
		require.False(t, *project2.PathEncryption)

		p2EncPass, p2EncKeyID, err := projectDB.GetEncryptedPassphrase(userCtx, project2.ID)
		require.NoError(t, err)
		// encryptedPassphrase should not be empty because project encryption is managed by satellite
		require.NotEmpty(t, p2EncPass)
		require.NotNil(t, p2EncKeyID)

		p2Pass, err := kmsService.DecryptPassphrase(ctx, *p2EncKeyID, p2EncPass)
		require.NoError(t, err)

		config, err = srv.GetProjectConfig(userCtx, project2.ID)
		require.NoError(t, err)
		require.Equal(t, string(p2Pass), config.Passphrase)

		key1 := *p2EncKeyID
		require.Equal(t, sat.Config.KeyManagement.DefaultMasterKey, key1)

		// change default key
		key2 := 2
		sat.Config.KeyManagement.DefaultMasterKey = key2
		require.NotEqual(t, key1, key2)

		*kmsService = *kms.NewService(sat.Config.KeyManagement)
		require.NoError(t, kmsService.Initialize(ctx))

		// create new project
		project3, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project3",
			ManagePassphrase: true,
		})
		require.NoError(t, err)
		require.NotNil(t, project3.PathEncryption)
		require.False(t, *project3.PathEncryption)

		// verify new default key is used for passphrase encryption
		p3EncPass, p3EncKeyID, err := projectDB.GetEncryptedPassphrase(userCtx, project3.ID)
		require.NoError(t, err)
		require.NotEmpty(t, p3EncPass)
		require.NotNil(t, p3EncKeyID)
		require.Equal(t, key2, *p3EncKeyID)

		p3Pass, err := kmsService.DecryptPassphrase(ctx, *p3EncKeyID, p3EncPass)
		require.NoError(t, err)

		config, err = srv.GetProjectConfig(userCtx, project3.ID)
		require.NoError(t, err)
		require.Equal(t, string(p3Pass), config.Passphrase)

		// verify previous project still returns previous default key and passphrase can be decrypted by it
		p2EncPass, p2EncKeyID, err = projectDB.GetEncryptedPassphrase(userCtx, project2.ID)
		require.NoError(t, err)
		require.NotEmpty(t, p2EncPass)
		require.NotNil(t, p2EncKeyID)
		require.Equal(t, key1, *p2EncKeyID)

		// double check decrypted project2 passphrase is the same as before
		pass, err := kmsService.DecryptPassphrase(ctx, *p2EncKeyID, p2EncPass)
		require.NoError(t, err)

		require.Equal(t, p2Pass, pass)
	})
}

func TestSatelliteManagedProjectWithDisabled(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SatelliteManagedEncryptionEnabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service
		// the kms service should not be up because SatelliteManagedEncryptionEnabled is disabled
		// and no KMS config was provided.
		require.Nil(t, sat.API.KeyManagement.Service)
		projectDB := sat.DB.Console().Projects()

		existingUser, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, existingUser.ID)
		require.NoError(t, err)

		// creating a managed project should fail because satellite managed encryption is disabled
		_, err = srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project",
			ManagePassphrase: true,
		})
		require.True(t, errs.Is(err, console.ErrSatelliteManagedEncryption))

		srv.TestToggleSatelliteManagedEncryption(true)
		_, err = srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project",
			ManagePassphrase: true,
		})
		require.True(t, errs.Is(err, console.ErrSatelliteManagedEncryption))
		srv.TestToggleSatelliteManagedEncryption(false)

		project, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "Test Project",
		})
		require.NoError(t, err)

		project.PassphraseEnc = []byte("test-passphrase-enc")
		err = projectDB.Update(userCtx, project)
		require.NoError(t, err)

		config, err := srv.GetProjectConfig(userCtx, project.ID)
		require.NoError(t, err)
		require.Empty(t, config.Passphrase)
	})
}

func TestSatelliteManagedProjectWithDisabledAndConfig(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,

		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SatelliteManagedEncryptionEnabled = false
				config.Console.ManagedEncryption.PathEncryptionEnabled = false
				config.KeyManagement.KeyInfos = kms.KeyInfos{
					Values: map[int]kms.KeyInfo{
						1: {
							SecretVersion: "secretversion1", SecretChecksum: 12345,
						},
						2: {
							SecretVersion: "secretversion2", SecretChecksum: 54321,
						},
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		srv := sat.API.Console.Service
		kmsService := sat.API.KeyManagement.Service
		// the kms service should be up even though satellite managed encryption is disabled
		// because KMS config was provided.
		require.NotNil(t, kmsService)
		projectDB := sat.DB.Console().Projects()

		existingUser, _, err := srv.GetUserByEmailWithUnverified(ctx, planet.Uplinks[0].User[sat.ID()].Email)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, existingUser.ID)
		require.NoError(t, err)

		// creating a managed project should fail because satellite managed encryption is disabled
		_, err = srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project",
			ManagePassphrase: true,
		})
		require.True(t, errs.Is(err, console.ErrSatelliteManagedEncryption))

		srv.TestToggleSatelliteManagedEncryption(true)
		project, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name:             "Test Project",
			ManagePassphrase: true,
		})
		require.NoError(t, err)
		require.NotNil(t, project)
		require.False(t, *project.PathEncryption)

		srv.TestToggleSatelliteManagedEncryption(false)

		encryptedPassphrase, _, err := projectDB.GetEncryptedPassphrase(userCtx, project.ID)
		require.NoError(t, err)
		// encryptedPassphrase should not be empty because project encryption is managed by satellite
		require.NotEmpty(t, encryptedPassphrase)

		// should be able to get passphrase of already created satellite managed project
		config, err := srv.GetProjectConfig(userCtx, project.ID)
		require.NoError(t, err)
		require.NotEmpty(t, config.Passphrase)

		project2, err := srv.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "Test Project2",
		})
		require.NoError(t, err)
		require.NotNil(t, project2)
		require.True(t, *project2.PathEncryption)

		config, err = srv.GetProjectConfig(userCtx, project2.ID)
		require.NoError(t, err)
		require.Empty(t, config.Passphrase)
	})
}

func TestPaymentsWalletPayments(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.BillingConfig.DisableLoop = false
				config.Payments.BonusRate = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		now := time.Now().Truncate(time.Second).UTC()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
			Password: "example",
		}, 1)
		require.NoError(t, err)

		wallet := blockchaintest.NewAddress()
		err = sat.DB.Wallets().Add(ctx, user.ID, wallet)
		require.NoError(t, err)

		var transactions []stripe.Transaction
		for i := 0; i < 5; i++ {
			tx := stripe.Transaction{
				ID:        coinpayments.TransactionID(strconv.Itoa(i)),
				AccountID: user.ID,
				Address:   blockchaintest.NewAddress().Hex(),
				Amount:    currency.AmountFromBaseUnits(1000000000, currency.StorjToken),
				Received:  currency.AmountFromBaseUnits(1000000000, currency.StorjToken),
				Status:    coinpayments.StatusCompleted,
				Key:       "key",
				Timeout:   0,
			}

			createdAt, err := sat.DB.StripeCoinPayments().Transactions().TestInsert(ctx, tx)
			require.NoError(t, err)
			err = sat.DB.StripeCoinPayments().Transactions().TestLockRate(ctx, tx.ID, decimal.NewFromInt(1))
			require.NoError(t, err)

			tx.CreatedAt = createdAt.UTC()
			transactions = append(transactions, tx)
		}

		var cachedPayments []storjscan.CachedPayment
		for i := 0; i < 10; i++ {
			chainID := billing.SourceChainIDs[billing.StorjScanZkSyncSource]
			if i%2 == 0 {
				chainID = billing.SourceChainIDs[billing.StorjScanEthereumSource]
			}
			cachedPayments = append(cachedPayments, storjscan.CachedPayment{
				ChainID:     chainID[0],
				From:        blockchaintest.NewAddress(),
				To:          wallet,
				TokenValue:  currency.AmountFromBaseUnits(1000, currency.StorjToken),
				USDValue:    currency.AmountFromBaseUnits(1000, currency.USDollarsMicro),
				Status:      payments.PaymentStatusConfirmed,
				BlockHash:   blockchaintest.NewHash(),
				BlockNumber: int64(i),
				Transaction: blockchaintest.NewHash(),
				LogIndex:    0,
				Timestamp:   now.Add(time.Duration(i) * 15 * time.Second),
			})
		}
		err = sat.DB.StorjscanPayments().InsertBatch(ctx, cachedPayments)
		require.NoError(t, err)

		reqCtx := console.WithUser(ctx, user)

		sort.Slice(cachedPayments, func(i, j int) bool {
			if cachedPayments[i].ChainID != cachedPayments[j].ChainID {
				return cachedPayments[i].ChainID > cachedPayments[j].ChainID
			}
			if cachedPayments[i].BlockNumber != cachedPayments[j].BlockNumber {
				return cachedPayments[i].BlockNumber > cachedPayments[j].BlockNumber
			}
			return cachedPayments[i].LogIndex > cachedPayments[j].LogIndex
		})
		sort.Slice(transactions, func(i, j int) bool {
			return transactions[i].CreatedAt.After(transactions[j].CreatedAt)
		})

		paymentSourceChainIDs := make(map[int64]string)
		for source, IDs := range billing.SourceChainIDs {
			for _, ID := range IDs {
				paymentSourceChainIDs[ID] = source
			}
		}
		var expected []console.PaymentInfo
		for _, pmnt := range cachedPayments {
			expected = append(expected, console.PaymentInfo{
				ID:        fmt.Sprintf("%s#%d", pmnt.Transaction.Hex(), pmnt.LogIndex),
				Type:      "storjscan",
				Wallet:    pmnt.To.Hex(),
				Amount:    pmnt.USDValue,
				Status:    string(pmnt.Status),
				Link:      sat.API.Console.Service.Payments().BlockExplorerURL(pmnt.Transaction.Hex(), paymentSourceChainIDs[pmnt.ChainID]),
				Timestamp: pmnt.Timestamp,
			})
		}
		for _, tx := range transactions {
			expected = append(expected, console.PaymentInfo{
				ID:        tx.ID.String(),
				Type:      "coinpayments",
				Wallet:    tx.Address,
				Amount:    currency.AmountFromBaseUnits(1000, currency.USDollars),
				Received:  currency.AmountFromBaseUnits(1000, currency.USDollars),
				Status:    tx.Status.String(),
				Link:      coinpayments.GetCheckoutURL(tx.Key, tx.ID),
				Timestamp: tx.CreatedAt,
			})
		}

		// get billing chore to insert bonuses for transactions.
		sat.Core.Payments.BillingChore.TransactionCycle.TriggerWait()

		txns, err := sat.DB.Billing().ListSource(ctx, user.ID, billing.StorjScanBonusSource)
		require.NoError(t, err)
		require.NotEmpty(t, txns)

		for _, txn := range txns {
			if txn.Source != billing.StorjScanBonusSource {
				continue
			}
			var meta struct {
				ReferenceID string
				Wallet      string
				LogIndex    int
			}
			err = json.NewDecoder(bytes.NewReader(txn.Metadata)).Decode(&meta)
			require.NoError(t, err)

			expected = append(expected, console.PaymentInfo{
				ID:        strconv.FormatInt(txn.ID, 10),
				Type:      txn.Source,
				Wallet:    meta.Wallet,
				Amount:    txn.Amount,
				Status:    string(txn.Status),
				Link:      sat.API.Console.Service.Payments().BlockExplorerURL(meta.ReferenceID, txn.Source),
				Timestamp: txn.Timestamp,
			})
		}

		walletPayments, err := sat.API.Console.Service.Payments().WalletPayments(reqCtx)
		require.NoError(t, err)
		require.Equal(t, expected, walletPayments.Payments)
	})
}

type mockDepositWallets struct {
	address  blockchain.Address
	payments []payments.WalletPaymentWithConfirmations
}

func (dw mockDepositWallets) Claim(_ context.Context, _ uuid.UUID) (blockchain.Address, error) {
	return dw.address, nil
}

func (dw mockDepositWallets) Get(_ context.Context, _ uuid.UUID) (blockchain.Address, error) {
	return dw.address, nil
}

func (dw mockDepositWallets) Payments(
	_ context.Context,
	_ blockchain.Address,
	_ int,
	_ int64,
) (p []payments.WalletPayment, err error) {
	return
}

func (dw mockDepositWallets) PaymentsWithConfirmations(
	_ context.Context,
	_ blockchain.Address,
) ([]payments.WalletPaymentWithConfirmations, error) {
	return dw.payments, nil
}

func TestWalletPaymentsWithConfirmations(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		paymentsService := service.Payments()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
			Password: "example",
		}, 1)
		require.NoError(t, err)

		now := time.Now()
		wallet := blockchaintest.NewAddress()

		var expected []payments.WalletPaymentWithConfirmations
		for i := 0; i < 3; i++ {
			expected = append(expected, payments.WalletPaymentWithConfirmations{
				From:          blockchaintest.NewAddress().Hex(),
				To:            wallet.Hex(),
				TokenValue:    currency.AmountFromBaseUnits(int64(i), currency.StorjToken).AsDecimal(),
				USDValue:      currency.AmountFromBaseUnits(int64(i), currency.USDollarsMicro).AsDecimal(),
				Status:        payments.PaymentStatusConfirmed,
				BlockHash:     blockchaintest.NewHash().Hex(),
				BlockNumber:   int64(i),
				Transaction:   blockchaintest.NewHash().Hex(),
				LogIndex:      i,
				Timestamp:     now,
				Confirmations: int64(i),
				BonusTokens:   decimal.NewFromInt(int64(i)),
			})
		}

		paymentsService.TestSwapDepositWallets(mockDepositWallets{address: wallet, payments: expected})

		reqCtx := console.WithUser(ctx, user)

		walletPayments, err := paymentsService.WalletPaymentsWithConfirmations(reqCtx)
		require.NoError(t, err)
		require.NotZero(t, len(walletPayments))

		for i, wp := range walletPayments {
			require.Equal(t, expected[i].From, wp.From)
			require.Equal(t, expected[i].To, wp.To)
			require.Equal(t, expected[i].TokenValue, wp.TokenValue)
			require.Equal(t, expected[i].USDValue, wp.USDValue)
			require.Equal(t, expected[i].Status, wp.Status)
			require.Equal(t, expected[i].BlockHash, wp.BlockHash)
			require.Equal(t, expected[i].BlockNumber, wp.BlockNumber)
			require.Equal(t, expected[i].Transaction, wp.Transaction)
			require.Equal(t, expected[i].LogIndex, wp.LogIndex)
			require.Equal(t, expected[i].Timestamp, wp.Timestamp)
		}
	})
}

func TestPaymentsPurchase(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.PackagePlans.Packages = map[string]payments.PackagePlan{
					"partner": {Price: 1000, Credit: 1500},
				}
				config.Console.UpgradePayUpfrontAmount = 500
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		p := sat.API.Console.Service.Payments()
		stripeClient := sat.API.Payments.StripeClient

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Test User",
			Email:     "test@mail.test",
			UserAgent: []byte("partner"),
		}, 1)
		require.NoError(t, err)
		require.True(t, user.IsFree())

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		pm, err := stripeClient.PaymentMethods().New(&stripeLib.PaymentMethodParams{
			Type: stripeLib.String(string(stripeLib.PaymentMethodTypeCard)),
			Card: &stripeLib.PaymentMethodCardParams{
				Token:    stripeLib.String("test"),
				ExpYear:  stripeLib.Int64(int64(2050)),
				ExpMonth: stripeLib.Int64(int64(05)),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, pm)

		params := payments.PurchaseParams{
			Intent: payments.PurchaseIntent(999),
			AddCardParams: payments.AddCardParams{
				Token: pm.ID,
			},
		}

		err = p.Purchase(userCtx, &params)
		require.NoError(t, err)

		user, err = sat.DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, user.IsFree())

		params.Intent = payments.PurchasePackageIntent
		err = p.Purchase(userCtx, &params)
		require.NoError(t, err)

		user, err = sat.DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, user.IsPaid())

		user, err = sat.AddUser(ctx, console.CreateUser{
			FullName: "Another User",
			Email:    "test1@mail.test",
		}, 1)
		require.NoError(t, err)
		require.True(t, user.IsFree())

		userCtx, err = sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		params.Intent = payments.PurchaseUpgradedAccountIntent
		err = p.Purchase(userCtx, &params)
		require.NoError(t, err)

		user, err = sat.DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, user.IsPaid())
	})
}

func TestPaymentsPurchasePreexistingInvoice(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.PackagePlans.Packages = map[string]payments.PackagePlan{
					"partner": {Price: 1000, Credit: 1500},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		p := sat.API.Console.Service.Payments()
		stripeClient := sat.API.Payments.StripeClient

		partner := "partner"

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName:  "Test User",
			Email:     "test@mail.test",
			UserAgent: []byte(partner),
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		pm, err := stripeClient.PaymentMethods().New(&stripeLib.PaymentMethodParams{
			Type: stripeLib.String(string(stripeLib.PaymentMethodTypeCard)),
			Card: &stripeLib.PaymentMethodCardParams{
				Token:    stripeLib.String(stripe.MockInvoicesPaySuccess),
				ExpYear:  stripeLib.Int64(int64(2050)),
				ExpMonth: stripeLib.Int64(int64(05)),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, pm)

		invs, err := sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 0)

		// test purchase with draft invoice
		invoiceDesc := partner + " package plan"

		inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(ctx, user.ID, 1000, invoiceDesc)
		require.NoError(t, err)
		require.Equal(t, payments.InvoiceStatusDraft, inv.Status)

		draftInv := inv.ID

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
		require.Equal(t, draftInv, invs[0].ID)

		params := payments.PurchaseParams{
			Intent: payments.PurchasePackageIntent,
			AddCardParams: payments.AddCardParams{
				Token: pm.ID,
			},
		}

		require.NoError(t, p.Purchase(userCtx, &params))

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
		require.NotEqual(t, draftInv, invs[0].ID)
		require.Equal(t, payments.InvoiceStatusPaid, invs[0].Status)

		_, err = sat.API.Payments.StripeService.Accounts().Invoices().Delete(ctx, invs[0].ID)
		require.NoError(t, err)

		// test purchase with open invoice
		inv, err = sat.API.Payments.StripeService.Accounts().Invoices().Create(ctx, user.ID, 1000, invoiceDesc)
		require.NoError(t, err)

		// attempting to pay a draft invoice changes it to open if payment fails
		_, err = sat.API.Payments.StripeService.Accounts().Invoices().Pay(ctx, inv.ID, stripe.MockInvoicesPayFailure)
		require.Error(t, err)

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
		require.Equal(t, payments.InvoiceStatusOpen, invs[0].Status)

		require.NoError(t, p.Purchase(userCtx, &params))

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
		require.Equal(t, payments.InvoiceStatusPaid, invs[0].Status)

		// purchase with paid invoice skips creating and or paying invoice
		require.NoError(t, p.Purchase(userCtx, &params))

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
	})
}

func TestServiceGenMethods(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		s := sat.API.Console.Service
		u0 := planet.Uplinks[0]
		u1 := planet.Uplinks[1]
		user0Ctx, err := sat.UserContext(ctx, u0.Projects[0].Owner.ID)
		require.NoError(t, err)
		user1Ctx, err := sat.UserContext(ctx, u1.Projects[0].Owner.ID)
		require.NoError(t, err)

		p0ID := u0.Projects[0].ID
		p, err := s.GetProject(user1Ctx, u1.Projects[0].ID)
		require.NoError(t, err)
		p1PublicID := p.PublicID

		// add a credit card to put the user in the paid tier
		_, err = sat.API.Console.Service.Payments().AddCreditCard(user0Ctx, "test-cc-token")
		require.NoError(t, err)
		user0Ctx, err = sat.UserContext(ctx, u0.Projects[0].Owner.ID)
		require.NoError(t, err)

		_, err = sat.API.Console.Service.Payments().AddCreditCard(user1Ctx, "test-cc-token")
		require.NoError(t, err)
		user1Ctx, err = sat.UserContext(ctx, u1.Projects[0].Owner.ID)
		require.NoError(t, err)

		for _, tt := range []struct {
			name   string
			ID     uuid.UUID
			ctx    context.Context
			uplink *testplanet.Uplink
		}{
			{"projectID", p0ID, user0Ctx, u0},
			{"publicID", p1PublicID, user1Ctx, u1},
		} {

			t.Run("GenUpdateProject with "+tt.name, func(t *testing.T) {
				updatedName := "name " + tt.name
				updatedDescription := "desc " + tt.name
				updatedStorageLimit := memory.Size(100)
				updatedBandwidthLimit := memory.Size(100)

				info := console.UpsertProjectInfo{
					Name:           updatedName,
					Description:    updatedDescription,
					StorageLimit:   &updatedStorageLimit,
					BandwidthLimit: &updatedBandwidthLimit,
				}
				updatedProject, err := s.GenUpdateProject(tt.ctx, tt.ID, info)
				require.NoError(t, err.Err)
				if tt.name == "projectID" {
					require.Equal(t, tt.ID, updatedProject.ID)
				} else {
					require.Equal(t, tt.ID, updatedProject.PublicID)
				}
				require.Equal(t, info.Name, updatedProject.Name)
				require.Equal(t, info.Description, updatedProject.Description)
				require.Equal(t, info.StorageLimit, updatedProject.UserSpecifiedStorageLimit)
				require.Equal(t, info.BandwidthLimit, updatedProject.UserSpecifiedBandwidthLimit)
			})
			t.Run("GenCreateAPIKey with "+tt.name, func(t *testing.T) {
				request := console.CreateAPIKeyRequest{
					ProjectID: tt.ID.String(),
					Name:      tt.name + " Key",
				}
				apiKey, err := s.GenCreateAPIKey(tt.ctx, request)
				require.NoError(t, err.Err)
				require.Equal(t, tt.ID, apiKey.KeyInfo.ProjectID)
				require.Equal(t, request.Name, apiKey.KeyInfo.Name)
			})
			t.Run("GenGetAPIKeys with "+tt.name, func(t *testing.T) {
				apiKeys, err := s.GenGetAPIKeys(tt.ctx, tt.ID, "", 10, 1, 0, 0)
				require.NoError(t, err.Err)
				require.NotEmpty(t, apiKeys)
				for _, key := range apiKeys.APIKeys {
					require.Equal(t, tt.ID, key.ProjectID)
				}
			})

			bucket := "testbucket"
			require.NoError(t, tt.uplink.TestingCreateBucket(tt.ctx, sat, bucket))
			require.NoError(t, tt.uplink.Upload(tt.ctx, sat, bucket, "helloworld.txt", []byte("hello world")))
			sat.Accounting.Tally.Loop.TriggerWait()

			t.Run("GenGetSingleBucketUsageRollup with "+tt.name, func(t *testing.T) {
				rollup, err := s.GenGetSingleBucketUsageRollup(tt.ctx, tt.ID, bucket, time.Now().Add(-24*time.Hour), time.Now())
				require.NoError(t, err.Err)
				require.NotNil(t, rollup)
				require.Equal(t, tt.ID, rollup.ProjectID)
			})
			t.Run("GenGetBucketUsageRollups with "+tt.name, func(t *testing.T) {
				rollups, err := s.GenGetBucketUsageRollups(tt.ctx, tt.ID, time.Now().Add(-24*time.Hour), time.Now())
				require.NoError(t, err.Err)
				require.NotEmpty(t, rollups)
				for _, r := range rollups {
					require.Equal(t, tt.ID, r.ProjectID)
				}
			})

			// create empty project for easy deletion
			p, err := s.CreateProject(tt.ctx, console.UpsertProjectInfo{
				Name:        "foo",
				Description: "bar",
			})
			require.NoError(t, err)

			t.Run("GenDeleteProject with "+tt.name, func(t *testing.T) {
				var id uuid.UUID
				if tt.name == "projectID" {
					id = p.ID
				} else {
					id = p.PublicID
				}
				httpErr := s.GenDeleteProject(tt.ctx, id)
				require.NoError(t, httpErr.Err)
				p, err := s.GetProject(ctx, id)
				require.Error(t, err)
				require.Nil(t, p)
			})
		}
	})
}

type EmailVerifier struct {
	Data    consoleapi.ContextChannel
	Context context.Context
}

func (v *EmailVerifier) SendEmail(ctx context.Context, msg *post.Message) error {
	body := ""
	for _, part := range msg.Parts {
		body += part.Content
	}
	return v.Data.Send(v.Context, body)
}

func (v *EmailVerifier) FromAddress() post.Address {
	return post.Address{}
}

func TestProjectInvitations(t *testing.T) {
	testplanet.Run(t, testplanet.Config{SatelliteCount: 1, NonParallel: true}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		invitesDB := sat.DB.Console().ProjectInvitations()

		addUser := func(t *testing.T, ctx context.Context) *console.User {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    fmt.Sprintf("%s@mail.test", testrand.RandAlphaNumeric(16)),
			}, 1)
			require.NoError(t, err)
			return user
		}

		getUserAndCtx := func(t *testing.T) (*console.User, context.Context) {
			ctx := testcontext.New(t)
			user := addUser(t, ctx)
			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			return user, userCtx
		}

		addProject := func(t *testing.T, ctx context.Context) *console.Project {
			owner := addUser(t, ctx)
			project, err := sat.AddProject(ctx, owner.ID, "Test Project")
			require.NoError(t, err)
			return project
		}

		addInvite := func(t *testing.T, ctx context.Context, project *console.Project, email string) *console.ProjectInvitation {
			invite, err := invitesDB.Upsert(ctx, &console.ProjectInvitation{
				ProjectID: project.ID,
				Email:     email,
				InviterID: &project.OwnerID,
			})
			require.NoError(t, err)

			return invite
		}

		upgradeToPaidTier := func(t *testing.T, ctx context.Context, user *console.User) context.Context {
			kind := console.PaidUser
			err := sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &kind})
			require.NoError(t, err)
			ctx, err = sat.UserContext(ctx, user.ID)
			require.NoError(t, err)
			return ctx
		}

		setInviteDate := func(t *testing.T, ctx context.Context, invite *console.ProjectInvitation, createdAt time.Time) {
			db := sat.DB.Testing()
			result, err := db.RawDB().ExecContext(ctx,
				db.Rebind("UPDATE project_invitations SET created_at = ? WHERE project_id = ? AND email = ?"),
				createdAt, invite.ProjectID, strings.ToUpper(invite.Email),
			)
			require.NoError(t, err)

			count, err := result.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 1, count)

			newInvite, err := invitesDB.Get(ctx, invite.ProjectID, invite.Email)
			require.NoError(t, err)
			*invite = *newInvite
		}

		t.Run("invite and reinvite users", func(t *testing.T) {
			user, ctx := getUserAndCtx(t)
			user2, ctx2 := getUserAndCtx(t)

			project, err := sat.AddProject(ctx, user.ID, "Test Project")
			require.NoError(t, err)

			// expect reinvitation to fail due to lack of preexisting invitation record.
			invites, err := service.ReinviteProjectMembers(ctx, project.ID, []string{user2.Email})
			require.True(t, console.ErrProjectInviteInvalid.Has(err))
			require.Empty(t, invites)

			invite, err := service.InviteNewProjectMember(ctx, project.ID, user2.Email)
			require.NoError(t, err)
			require.NotNil(t, invite)

			invites, err = service.GetUserProjectInvitations(ctx2)
			require.NoError(t, err)
			require.Len(t, invites, 1)

			// adding in a non-existent user should work.
			_, err = service.InviteNewProjectMember(ctx, project.ID, "notauser@mail.test")
			require.NoError(t, err)

			// prevent unauthorized users from inviting others (user2 is not a member of the project yet).
			const testEmail = "other@mail.test"
			ctx2 = upgradeToPaidTier(t, ctx2, user2)
			_, err = service.InviteNewProjectMember(ctx2, project.ID, testEmail)
			require.Error(t, err)
			require.True(t, console.ErrNoMembership.Has(err))

			require.NoError(t, service.RespondToProjectInvitation(ctx2, project.ID, console.ProjectInvitationAccept))

			pm2, err := service.UpdateProjectMemberRole(ctx, user2.ID, project.ID, console.RoleAdmin)
			require.NoError(t, err)
			require.Equal(t, console.RoleAdmin, pm2.Role)

			// inviting a user with a preexisting invitation record should fail.
			_, err = service.InviteNewProjectMember(ctx2, project.ID, testEmail)
			require.NoError(t, err)
			_, err = service.InviteNewProjectMember(ctx2, project.ID, testEmail)
			require.True(t, console.ErrAlreadyInvited.Has(err))

			// reinviting a user with a preexisting, unexpired invitation record should fail.
			invites, err = service.ReinviteProjectMembers(ctx2, project.ID, []string{testEmail})
			require.True(t, console.ErrAlreadyInvited.Has(err))
			require.Empty(t, invites)

			// expire the invitation.
			user3Invite, err := invitesDB.Get(ctx, project.ID, testEmail)
			require.NoError(t, err)
			require.False(t, service.IsProjectInvitationExpired(user3Invite))
			oldCreatedAt := user3Invite.CreatedAt
			setInviteDate(t, ctx, user3Invite, time.Now().Add(-sat.Config.Console.ProjectInvitationExpiration))
			require.True(t, service.IsProjectInvitationExpired(user3Invite))

			// resending an expired invitation should succeed.
			invites, err = service.ReinviteProjectMembers(ctx2, project.ID, []string{testEmail})
			require.NoError(t, err)
			require.Len(t, invites, 1)
			require.Equal(t, user2.ID, *invites[0].InviterID)
			require.True(t, invites[0].CreatedAt.After(oldCreatedAt))

			// inviting a project member should fail.
			_, err = service.InviteNewProjectMember(ctx, project.ID, user2.Email)
			require.Error(t, err)

			// test inviting unverified user.
			sender := &EmailVerifier{Context: ctx}
			sat.API.Mail.Service.Sender = sender

			regToken, err := service.CreateRegToken(ctx, 1)
			require.NoError(t, err)

			unverified, err := service.CreateUser(ctx, console.CreateUser{
				FullName: "test user",
				Email:    "test-unverified-email@test",
				Password: "password",
			}, regToken.Secret)
			require.NoError(t, err)
			require.Zero(t, unverified.Status)

			invite, err = service.InviteNewProjectMember(ctx, project.ID, unverified.Email)
			require.NoError(t, err)
			require.Equal(t, unverified.Email, strings.ToLower(invite.Email))

			body, err := sender.Data.Get(ctx)
			require.NoError(t, err)
			require.Contains(t, body, "/activation")

			user3, ctx3 := getUserAndCtx(t)

			_, err = service.AddProjectMembers(ctx, project.ID, []string{user3.Email})
			require.NoError(t, err)

			// Members with console.RoleMember status can't invite other members.
			_, err = service.InviteNewProjectMember(ctx3, project.ID, "test@example.com")
			require.True(t, console.ErrForbidden.Has(err))
		})

		t.Run("get invitation", func(t *testing.T) {
			user, ctx := getUserAndCtx(t)

			invites, err := service.GetUserProjectInvitations(ctx)
			require.NoError(t, err)
			require.Empty(t, invites)

			invite := addInvite(t, ctx, addProject(t, ctx), user.Email)
			invites, err = service.GetUserProjectInvitations(ctx)
			require.NoError(t, err)
			require.Len(t, invites, 1)
			require.Equal(t, invite.ProjectID, invites[0].ProjectID)
			require.Equal(t, invite.Email, invites[0].Email)
			require.Equal(t, invite.InviterID, invites[0].InviterID)
			require.WithinDuration(t, invite.CreatedAt, invites[0].CreatedAt, time.Second)

			setInviteDate(t, ctx, &invites[0], time.Now().Add(-sat.Config.Console.ProjectInvitationExpiration))
			invites, err = service.GetUserProjectInvitations(ctx)
			require.NoError(t, err)
			require.Empty(t, invites)
		})

		t.Run("invite tokens", func(t *testing.T) {
			user, ctx1 := getUserAndCtx(t)

			project, err := sat.AddProject(ctx1, user.ID, "Test Project")
			require.NoError(t, err)

			someToken, err := service.CreateInviteToken(ctx1, project.PublicID, email, time.Now())
			require.NoError(t, err)
			require.NotEmpty(t, someToken)

			id, mail, err := service.ParseInviteToken(ctx1, someToken)
			require.NoError(t, err)
			require.Equal(t, project.PublicID, id)
			require.Equal(t, email, mail)

			someToken, err = service.CreateInviteToken(ctx1, project.PublicID, email, time.Now().Add(-360*time.Hour))
			require.NoError(t, err)
			require.NotEmpty(t, someToken)

			_, _, err = service.ParseInviteToken(ctx, someToken)
			require.Error(t, err)
			require.True(t, console.ErrTokenExpiration.Has(err))
		})

		t.Run("invite links", func(t *testing.T) {
			user, ctx1 := getUserAndCtx(t)
			user2, ctx2 := getUserAndCtx(t)

			project, err := sat.AddProject(ctx1, user.ID, "Test Project")
			require.NoError(t, err)

			_, err = service.GetInviteLink(ctx2, project.PublicID, user2.Email)
			require.Error(t, err)
			require.True(t, console.ErrNoMembership.Has(err))

			// no such project
			_, err = service.GetInviteLink(ctx1, testrand.UUID(), user2.Email)
			require.Error(t, err)
			require.ErrorIs(t, err, sql.ErrNoRows)

			// no invite exists.
			_, err = service.GetInviteLink(ctx1, project.PublicID, user2.Email)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			invite := addInvite(t, ctx1, project, user2.Email)

			someLink, err := service.GetInviteLink(ctx1, project.PublicID, user2.Email)
			require.NoError(t, err)
			require.NotEmpty(t, someLink)

			someToken, err := service.CreateInviteToken(ctx1, project.PublicID, user2.Email, invite.CreatedAt)
			require.NoError(t, err)
			require.NotEmpty(t, someToken)

			require.Contains(t, someLink, someToken)
		})

		t.Run("get invite by invite token", func(t *testing.T) {
			owner, ctx := getUserAndCtx(t)
			user, _ := getUserAndCtx(t)

			project, err := sat.AddProject(ctx, owner.ID, "Test Project")
			require.NoError(t, err)

			invite := addInvite(t, ctx, project, user.Email)

			someToken, err := service.CreateInviteToken(ctx, project.PublicID, "some@email.test", invite.CreatedAt)
			require.NoError(t, err)

			inviteFromToken, err := service.GetInviteByToken(ctx, someToken)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))
			require.Nil(t, inviteFromToken)

			inviteToken, err := service.CreateInviteToken(ctx, project.PublicID, user.Email, invite.CreatedAt)
			require.NoError(t, err)

			inviteFromToken, err = service.GetInviteByToken(ctx, inviteToken)
			require.NoError(t, err)
			require.NotNil(t, inviteFromToken)
			require.Equal(t, invite, inviteFromToken)

			setInviteDate(t, ctx, invite, time.Now().Add(-sat.Config.Console.ProjectInvitationExpiration))
			invites, err := service.GetUserProjectInvitations(ctx)
			require.NoError(t, err)
			require.Empty(t, invites)

			_, err = service.GetInviteByToken(ctx, inviteToken)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			// invalid project id. GetInviteByToken supports only public ids.
			someToken, err = service.CreateInviteToken(ctx, project.ID, user.Email, invite.CreatedAt)
			require.NoError(t, err)

			_, err = service.GetInviteByToken(ctx, someToken)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))
		})

		t.Run("accept invitation", func(t *testing.T) {
			user, ctx := getUserAndCtx(t)
			proj := addProject(t, ctx)

			invite := addInvite(t, ctx, proj, user.Email)

			// Expect an error when accepting an expired invitation.
			// The invitation should remain in the database.
			setInviteDate(t, ctx, invite, time.Now().Add(-sat.Config.Console.ProjectInvitationExpiration))
			err := service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationAccept)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			_, err = invitesDB.Get(ctx, proj.ID, user.Email)
			require.NoError(t, err)

			// Expect no error when accepting an active invitation.
			// The invitation should be removed from the database, and the user should be added as a member.
			setInviteDate(t, ctx, invite, time.Now())
			require.NoError(t, err)
			require.NoError(t, service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationAccept))

			_, err = invitesDB.Get(ctx, proj.ID, user.Email)
			require.ErrorIs(t, err, sql.ErrNoRows)

			memberships, err := sat.DB.Console().ProjectMembers().GetByMemberID(ctx, user.ID)
			require.NoError(t, err)
			require.Len(t, memberships, 1)
			require.Equal(t, proj.ID, memberships[0].ProjectID)

			// Ensure that accepting an invitation for a project you are already a member of doesn't return an error.
			// This is because the outcome of the operation is the same as if you weren't a member.
			require.NoError(t, service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationAccept))
			// Ensure that an error is returned if you're a member of a project whose invitation you decline.
			err = service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationDecline)
			require.True(t, console.ErrAlreadyMember.Has(err))
		})

		t.Run("reject invitation", func(t *testing.T) {
			user, ctx := getUserAndCtx(t)
			proj := addProject(t, ctx)

			invite := addInvite(t, ctx, proj, user.Email)

			// Expect an error when rejecting an expired invitation.
			// The invitation should remain in the database.
			setInviteDate(t, ctx, invite, time.Now().Add(-sat.Config.Console.ProjectInvitationExpiration))
			err := service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationDecline)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			_, err = invitesDB.Get(ctx, proj.ID, user.Email)
			require.NoError(t, err)

			// Expect no error when rejecting an active invitation.
			// The invitation should be removed from the database.
			setInviteDate(t, ctx, invite, time.Now())
			require.NoError(t, err)
			require.NoError(t, service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationDecline))

			_, err = invitesDB.Get(ctx, proj.ID, user.Email)
			require.ErrorIs(t, err, sql.ErrNoRows)

			memberships, err := sat.DB.Console().ProjectMembers().GetByMemberID(ctx, user.ID)
			require.NoError(t, err)
			require.Empty(t, memberships)

			// Ensure that declining an invitation for a project you are not a member of doesn't return an error.
			// This is because the outcome of the operation is the same as if you were a member.
			require.NoError(t, service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationDecline))
			// Ensure that an error is returned if you try to accept an invitation that you have already declined or doesn't exist.
			err = service.RespondToProjectInvitation(ctx, proj.ID, console.ProjectInvitationAccept)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))
		})

		t.Run("respond by bot account", func(t *testing.T) {
			user := addUser(t, ctx)
			botStatus := console.PendingBotVerification
			err := sat.API.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &botStatus,
			})
			require.NoError(t, err)

			proj := addProject(t, ctx)
			_ = addInvite(t, ctx, proj, user.Email)

			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			err = service.RespondToProjectInvitation(userCtx, proj.ID, console.ProjectInvitationDecline)
			require.Error(t, err)
			require.True(t, console.ErrBotUser.Has(err))
		})

		// test inviting a user and removing inviter before invite is accepted
		t.Run("remove inviter before invite accepted", func(t *testing.T) {
			owner, ownerCtx := getUserAndCtx(t)
			admin, adminCtx := getUserAndCtx(t)
			invitee, inviteeCtx := getUserAndCtx(t)

			project, err := sat.AddProject(ownerCtx, owner.ID, "Test Project")
			require.NoError(t, err)

			_, err = service.InviteNewProjectMember(ownerCtx, project.ID, admin.Email)
			require.NoError(t, err)

			require.NoError(t, service.RespondToProjectInvitation(adminCtx, project.ID, console.ProjectInvitationAccept))

			_, err = service.UpdateProjectMemberRole(ownerCtx, admin.ID, project.ID, console.RoleAdmin)
			require.NoError(t, err)

			// invitee is invited by admin
			_, err = service.InviteNewProjectMember(adminCtx, project.ID, invitee.Email)
			require.NoError(t, err)

			// remove admin from project before invitee accepts invite
			err = service.DeleteProjectMembersAndInvitations(ownerCtx, project.ID, []string{admin.Email})
			require.NoError(t, err)

			// invitee cannot accept invite since inviter has been removed

			someToken, err := service.CreateInviteToken(ctx, project.PublicID, invitee.Email, invitee.CreatedAt)
			require.NoError(t, err)

			_, err = service.GetInviteByToken(ctx, someToken)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			err = service.RespondToProjectInvitation(inviteeCtx, project.ID, console.ProjectInvitationAccept)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			// add admin again but set status to deleted
			_, err = service.InviteNewProjectMember(ownerCtx, project.ID, admin.Email)
			require.NoError(t, err)

			require.NoError(t, service.RespondToProjectInvitation(adminCtx, project.ID, console.ProjectInvitationAccept))

			newStatus := console.Deleted
			err = sat.DB.Console().Users().Update(ctx, admin.ID, console.UpdateUserRequest{Status: &newStatus})
			require.NoError(t, err)

			// invitee cannot accept invite since inviter has been deleted

			_, err = service.GetInviteByToken(ctx, someToken)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			err = service.RespondToProjectInvitation(inviteeCtx, project.ID, console.ProjectInvitationAccept)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			// confirm that if inviter is re-added and active, invitee can accept invite

			newStatus = console.Active
			err = sat.DB.Console().Users().Update(ctx, admin.ID, console.UpdateUserRequest{Status: &newStatus})
			require.NoError(t, err)

			_, err = service.GetInviteByToken(ctx, someToken)
			require.NoError(t, err)

			err = service.RespondToProjectInvitation(inviteeCtx, project.ID, console.ProjectInvitationAccept)
			require.NoError(t, err)
		})
	})
}

func TestUserTenancy(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Console.EmailChangeFlowEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		invitesDB := sat.DB.Console().ProjectInvitations()

		updateContext := func(u *console.User) context.Context {
			userCtx, err := sat.UserContext(ctx, u.ID)
			require.NoError(t, err)
			return userCtx
		}

		registerAndActivateUser := func(email string, tenantID string) (context.Context, *console.User) {
			ctxWithTenant := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID})
			regToken1, err := service.CreateRegToken(ctx, 1)
			require.NoError(t, err)

			tenantUser, err := service.CreateUser(ctxWithTenant, console.CreateUser{
				FullName: email,
				Email:    email,
				Password: email,
			}, regToken1.Secret)
			require.NoError(t, err)
			require.NotNil(t, tenantUser.TenantID)
			require.Equal(t, tenantID, *tenantUser.TenantID)

			activationToken1, err := service.GenerateActivationToken(ctxWithTenant, tenantUser.ID, tenantUser.Email)
			require.NoError(t, err)

			_, err = service.ActivateAccount(ctxWithTenant, activationToken1)
			require.NoError(t, err)

			tenantUserCtx := updateContext(tenantUser)

			_, err = service.Payments().SetupAccount(tenantUserCtx)
			require.NoError(t, err)

			return tenantUserCtx, tenantUser
		}

		t.Run("create and activate user in tenant", func(t *testing.T) {
			tenantID := "testtenant"
			ctxWithTenant := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID})

			// Create registration token
			regToken, err := service.CreateRegToken(ctx, 1)
			require.NoError(t, err)

			// Create user in tenant using CreateUser
			tenantUser, err := service.CreateUser(ctxWithTenant, console.CreateUser{
				FullName: "Tenant User",
				Email:    "tenantuser@mail.test",
				Password: "password123",
			}, regToken.Secret)
			require.NoError(t, err)
			require.NotNil(t, tenantUser)
			require.Equal(t, console.Inactive, tenantUser.Status)
			require.NotNil(t, tenantUser.TenantID)
			require.Equal(t, tenantID, *tenantUser.TenantID)

			// Generate activation token
			activationToken, err := service.GenerateActivationToken(ctxWithTenant, tenantUser.ID, tenantUser.Email)
			require.NoError(t, err)
			require.NotEmpty(t, activationToken)

			// Activate account
			activatedUser, err := service.ActivateAccount(ctxWithTenant, activationToken)
			require.NoError(t, err)
			require.NotNil(t, activatedUser)
			require.Equal(t, console.Active, activatedUser.Status)
			require.Equal(t, tenantUser.ID, activatedUser.ID)
			require.NotNil(t, activatedUser.TenantID)
			require.Equal(t, tenantID, *activatedUser.TenantID)
		})

		t.Run("tenant users with project invites", func(t *testing.T) {
			tenantID := "projecttenant"
			ctxWithTenant := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID})

			tenantUser1Ctx, tenantUser1 := registerAndActivateUser("TENANT1@MAIL.TEST", tenantID)
			tenantUser2Ctx, tenantUser2 := registerAndActivateUser("TENANT2@MAIL.TEST", tenantID)

			nonTenantUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Non-Tenant User",
				Email:    "NONTENANT@MAIL.TEST",
			}, 1)
			require.NoError(t, err)

			nonTenantUserCtx, err := sat.UserContext(ctx, nonTenantUser.ID)
			require.NoError(t, err)

			project, err := sat.AddProject(tenantUser1Ctx, tenantUser1.ID, "Tenant Project")
			require.NoError(t, err)

			invite, err := service.InviteNewProjectMember(tenantUser1Ctx, project.ID, tenantUser2.Email)
			require.NoError(t, err)
			require.NotNil(t, invite)
			require.Equal(t, project.ID, invite.ProjectID)
			require.Equal(t, tenantUser2.Email, invite.Email)

			// Tenant user 2 can accept the invitation
			err = service.RespondToProjectInvitation(tenantUser2Ctx, project.ID, console.ProjectInvitationAccept)
			require.NoError(t, err)

			// Verify tenant user 2 is now a member
			memberships, err := sat.DB.Console().ProjectMembers().GetByMemberID(ctxWithTenant, tenantUser2.ID)
			require.NoError(t, err)
			require.Len(t, memberships, 1)
			require.Equal(t, project.ID, memberships[0].ProjectID)

			// Tenant user can invite non-tenant user email (invitation created but cannot be accepted)
			_, err = service.InviteNewProjectMember(tenantUser1Ctx, project.ID, nonTenantUser.Email)
			require.NoError(t, err)

			// Verify invitation exists
			invite, err = invitesDB.Get(tenantUser1Ctx, project.ID, nonTenantUser.Email)
			require.NoError(t, err)
			require.NotNil(t, invite)

			// Non-tenant user cannot accept tenant project invitation
			err = service.RespondToProjectInvitation(nonTenantUserCtx, project.ID, console.ProjectInvitationAccept)
			require.Error(t, err)
			require.True(t, console.ErrProjectInviteInvalid.Has(err))

			// Verify non-tenant user is NOT a member
			memberships, err = sat.DB.Console().ProjectMembers().GetByMemberID(ctx, nonTenantUser.ID)
			require.NoError(t, err)
			require.Len(t, memberships, 0)
		})

		t.Run("login with tenant isolation", func(t *testing.T) {
			tenant1ID := "logintenant1"
			tenant2ID := "logintenant2"
			ctxWithTenant1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant1ID})
			ctxWithTenant2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant2ID})
			sharedEmail := "shareduser@login.test"
			password := sharedEmail

			// Create and activate user in tenant1
			_, tenant1User := registerAndActivateUser(sharedEmail, tenant1ID)

			// Create and activate user in tenant2 with same email
			_, tenant2User := registerAndActivateUser(sharedEmail, tenant2ID)

			// Both users should be able to login with same email in their respective tenants
			token1, err := service.Token(ctxWithTenant1, console.AuthUser{
				Email:    sharedEmail,
				Password: password,
			})
			require.NoError(t, err)
			require.NotNil(t, token1)

			// Verify the token is for tenant1 user
			_, user1FromToken, err := service.TokenAuth(ctxWithTenant1, token1.Token, time.Now())
			require.NoError(t, err)
			require.Equal(t, tenant1User.ID, user1FromToken.UserID)

			token2, err := service.Token(ctxWithTenant2, console.AuthUser{
				Email:    sharedEmail,
				Password: password,
			})
			require.NoError(t, err)
			require.NotNil(t, token2)

			// Verify the token is for tenant2 user
			_, user2FromToken, err := service.TokenAuth(ctxWithTenant2, token2.Token, time.Now())
			require.NoError(t, err)
			require.Equal(t, tenant2User.ID, user2FromToken.UserID)

			// Verify the two logins returned different users
			require.NotEqual(t, user1FromToken.UserID, user2FromToken.UserID)

			// verify that login to another tenant fails
			someOtherTenantCtx := tenancy.WithContext(ctx, &tenancy.Context{TenantID: "someothertenant"})
			_, err = service.Token(someOtherTenantCtx, console.AuthUser{
				Email:    sharedEmail,
				Password: password,
			})
			require.True(t, console.ErrLoginCredentials.Has(err))
		})

		t.Run("change email with tenant isolation", func(t *testing.T) {
			tenant1ID := "emailtenant1"
			tenant2ID := "emailtenant2"
			ctxWithTenant1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant1ID})

			// Create and activate user in tenant1
			tenant1UserCtx, tenant1User := registerAndActivateUser("tenant1email@mail.test", tenant1ID)

			// Create and activate user in tenant2
			_, tenant2User := registerAndActivateUser("tenant2email@mail.test", tenant2ID)

			targetEmail := "tenant2email@mail.test"

			err := service.ChangeEmail(tenant1UserCtx, console.VerifyAccountPasswordStep, tenant1User.Email)
			require.NoError(t, err)

			user1, err := sat.DB.Console().Users().Get(ctxWithTenant1, tenant1User.ID)
			require.NoError(t, err)
			require.NotNil(t, user1.ActivationCode)

			tenant1UserCtx = updateContext(tenant1User)
			err = service.ChangeEmail(tenant1UserCtx, console.VerifyAccountEmailStep, user1.ActivationCode)
			require.NoError(t, err)

			tenant1UserCtx = updateContext(tenant1User)
			err = service.ChangeEmail(tenant1UserCtx, console.ChangeAccountEmailStep, targetEmail)
			require.NoError(t, err)

			user1, err = sat.DB.Console().Users().Get(ctxWithTenant1, tenant1User.ID)
			require.NoError(t, err)
			require.NotNil(t, user1.NewUnverifiedEmail)
			require.Equal(t, targetEmail, *user1.NewUnverifiedEmail)
			require.NotNil(t, user1.ActivationCode)

			tenant1UserCtx = updateContext(tenant1User)
			err = service.ChangeEmail(tenant1UserCtx, console.VerifyNewAccountEmailStep, user1.ActivationCode)
			require.NoError(t, err)

			// Verify that email was successfully changed because email uniqueness
			// is scoped to tenant
			user1, err = sat.DB.Console().Users().Get(ctx, tenant1User.ID)
			require.NoError(t, err)
			require.Equal(t, targetEmail, user1.Email)

			// Verify both users now have the same email but in different tenants
			user2, err := sat.DB.Console().Users().Get(ctx, tenant2User.ID)
			require.NoError(t, err)
			require.Equal(t, user1.Email, user2.Email)
			require.NotEqual(t, user1.ID, user2.ID)
			require.NotEqual(t, user1.TenantID, user2.TenantID)
		})
	})
}

func TestDelayedBotFreeze(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.Captcha.FlagBotsEnabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		captchaConfig := sat.Config.Console.Captcha
		accFreezeDB := sat.DB.Console().AccountFreezeEvents()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		user.SignupCaptcha = &captchaConfig.ScoreCutoffThreshold

		err = sat.API.Console.Service.SetAccountActive(ctx, user)
		require.NoError(t, err)

		event, err := accFreezeDB.Get(ctx, user.ID, console.DelayedBotFreeze)
		require.NoError(t, err)
		require.NotNil(t, event)
		require.GreaterOrEqual(t, *event.DaysTillEscalation, captchaConfig.MinFlagBotDelay)
		require.LessOrEqual(t, *event.DaysTillEscalation, captchaConfig.MaxFlagBotDelay)

		event, err = accFreezeDB.Get(ctx, user.ID, console.BotFreeze)
		require.True(t, errs.Is(err, sql.ErrNoRows))
		require.Nil(t, event)
	})
}

func TestGetValdiAPIKey(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.CloudGpusEnabled = true
				config.Valdi.SignRequests = false
				config.Valdi.SatelliteEmail = "storj@storj.test"
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user1, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[0].User[sat.ID()].Email, nil)
		require.NoError(t, err)
		p := planet.Uplinks[0].Projects[0]

		mockClient, transport := httpmock.NewClient()

		testValdiClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, sat.Config.Valdi.Config)
		require.NoError(t, err)

		*sat.API.Valdi.Client = *testValdiClient

		t.Run("non-existent project", func(t *testing.T) {
			userCtx, err := sat.UserContext(ctx, user1.ID)
			require.NoError(t, err)

			key, status, err := service.GetValdiAPIKey(userCtx, testrand.UUID())
			require.Error(t, err)
			require.Equal(t, http.StatusUnauthorized, status)
			require.Nil(t, key)
		})

		user2, err := sat.DB.Console().Users().GetByEmailAndTenant(ctx, planet.Uplinks[1].User[sat.ID()].Email, nil)
		require.NoError(t, err)

		t.Run("not project member", func(t *testing.T) {
			userCtx, err := sat.UserContext(ctx, user2.ID)
			require.NoError(t, err)

			key, status, err := service.GetValdiAPIKey(userCtx, p.PublicID)
			require.Error(t, err)
			require.Equal(t, http.StatusUnauthorized, status)
			require.Nil(t, key)
		})

		_, err = sat.DB.Console().ProjectMembers().Insert(ctx, user2.ID, p.ID, console.RoleMember)
		require.NoError(t, err)

		t.Run("not project owner", func(t *testing.T) {
			userCtx, err := sat.UserContext(ctx, user2.ID)
			require.NoError(t, err)

			key, status, err := service.GetValdiAPIKey(userCtx, p.PublicID)
			require.Error(t, err)
			require.Equal(t, http.StatusUnauthorized, status)
			require.Nil(t, key)
		})

		keySuccessResp := &valdiclient.CreateAPIKeyResponse{
			APIKey:            "1234",
			SecretAccessToken: "abc123",
		}

		tests := []struct {
			name string

			firstAPIKeyStatus int
			firstAPIKeyResp   interface{}

			createUserStatus int
			createUserError  *valdiclient.ErrorMessage

			secondAPIKeyStatus int
			secondAPIKeyResp   interface{}

			expectedStorjStatus int
			expectedStorjResp   interface{}
		}{
			{
				name: "create key errors with 404, then create user and create key succeed",

				firstAPIKeyStatus: http.StatusNotFound,
				firstAPIKeyResp:   &valdiclient.ErrorMessage{Detail: "user doesn't exist"},

				createUserStatus: http.StatusCreated,

				secondAPIKeyStatus: http.StatusOK,
				secondAPIKeyResp:   keySuccessResp,

				expectedStorjStatus: http.StatusOK,
				expectedStorjResp:   keySuccessResp,
			},
			{
				name: "create key errors with 404, then create user errors",

				firstAPIKeyStatus: http.StatusNotFound,
				firstAPIKeyResp:   &valdiclient.ErrorMessage{Detail: "user doesn't exist"},

				createUserStatus: http.StatusConflict,
				createUserError:  &valdiclient.ErrorMessage{Detail: "username already exists"},

				expectedStorjStatus: http.StatusConflict,
				expectedStorjResp:   "username already exists",
			},
			{
				name: "create key errors with 404, then create user succeeds, and create key errors",

				firstAPIKeyStatus: http.StatusNotFound,
				firstAPIKeyResp:   &valdiclient.ErrorMessage{Detail: "user doesn't exist"},

				createUserStatus: http.StatusCreated,

				secondAPIKeyStatus: http.StatusInternalServerError,
				secondAPIKeyResp:   &valdiclient.ErrorMessage{Detail: "some internal valdi error"},

				expectedStorjStatus: http.StatusInternalServerError,
				expectedStorjResp:   "some internal valdi error",
			},
			{
				name: "create key succeeds",

				firstAPIKeyStatus: http.StatusOK,
				firstAPIKeyResp:   keySuccessResp,

				expectedStorjStatus: http.StatusOK,
				expectedStorjResp:   keySuccessResp,
			},
			{
				name: "create key fails, not 404",

				firstAPIKeyStatus: http.StatusInternalServerError,
				firstAPIKeyResp: &valdiclient.ErrorMessage{
					Detail: "some internal valdi error",
				},

				expectedStorjStatus: http.StatusInternalServerError,
				expectedStorjResp:   "some internal valdi error",
			},
		}

		apiKeyEndpoint, err := url.JoinPath(sat.Config.Valdi.APIBaseURL, valdiclient.APIKeyPath)
		require.NoError(t, err)

		userEndpoint, err := url.JoinPath(sat.Config.Valdi.APIBaseURL, valdiclient.UserPath)
		require.NoError(t, err)

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body, err := json.Marshal(tt.firstAPIKeyResp)
				require.NoError(t, err)

				transport.AddResponse(apiKeyEndpoint, httpmock.Response{
					StatusCode: tt.firstAPIKeyStatus,
					Body:       string(body),
				})

				if tt.createUserStatus != 0 {
					body := ""
					if tt.createUserError != nil {
						errJSONData, err := json.Marshal(tt.createUserError)
						require.NoError(t, err)
						body = string(errJSONData)
					}

					transport.AddResponse(userEndpoint, httpmock.Response{
						StatusCode: tt.createUserStatus,
						Body:       body,
					})
				}

				if tt.secondAPIKeyStatus != 0 {
					keyJSONData, err := json.Marshal(tt.secondAPIKeyResp)
					require.NoError(t, err)

					transport.AddResponse(apiKeyEndpoint, httpmock.Response{
						StatusCode: tt.secondAPIKeyStatus,
						Body:       string(keyJSONData),
					})
				}

				userCtx, err := sat.UserContext(ctx, user1.ID)
				require.NoError(t, err)

				key, status, err := service.GetValdiAPIKey(userCtx, p.ID)
				require.Equal(t, tt.expectedStorjStatus, status)
				switch tt.expectedStorjStatus {
				case http.StatusOK:
					require.Equal(t, key, tt.expectedStorjResp)
				case http.StatusInternalServerError:
				default:
					require.Contains(t, err.Error(), tt.expectedStorjResp)
				}
			})
		}
	})
}

func TestStartFreeTrial(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.FreeTrialDuration = 30 * 24 * time.Hour // 30 days
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		p := service.Payments()

		t.Run("Member user can start free trial", func(t *testing.T) {
			memberUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName:          "Member User",
				Email:             "member@mail.test",
				NoTrialExpiration: true,
			}, 1)
			require.NoError(t, err)

			err = sat.DB.Console().Users().Update(ctx, memberUser.ID, console.UpdateUserRequest{
				Kind: func() *console.UserKind { k := console.MemberUser; return &k }(),
			})
			require.NoError(t, err)

			memberUser, err = sat.DB.Console().Users().Get(ctx, memberUser.ID)
			require.NoError(t, err)
			require.True(t, memberUser.IsMember())
			require.True(t, memberUser.TrialExpiration == nil || memberUser.TrialExpiration.IsZero())

			userCtx, err := sat.UserContext(ctx, memberUser.ID)
			require.NoError(t, err)

			err = p.StartFreeTrial(userCtx)
			require.NoError(t, err)

			// Verify user is now Free and has trial expiration set.
			user, err := sat.DB.Console().Users().Get(ctx, memberUser.ID)
			require.NoError(t, err)
			require.True(t, user.IsFree())
			require.NotNil(t, user.TrialExpiration)
			require.False(t, user.TrialExpiration.IsZero())
			require.True(t, user.TrialExpiration.After(time.Now()))
		})

		t.Run("Non-member user cannot start free trial", func(t *testing.T) {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Free User",
				Email:    "free@mail.test",
			}, 1)
			require.NoError(t, err)
			require.True(t, user.IsFree())

			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			err = p.StartFreeTrial(userCtx)
			require.Error(t, err)
			require.True(t, console.ErrUnauthorized.Has(err))

			err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				Kind: func() *console.UserKind { k := console.PaidUser; return &k }(),
			})
			require.NoError(t, err)

			userCtx, err = sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			err = p.StartFreeTrial(userCtx)
			require.Error(t, err)
			require.True(t, console.ErrUnauthorized.Has(err))

			err = sat.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
				Kind: func() *console.UserKind { k := console.NFRUser; return &k }(),
			})
			require.NoError(t, err)

			userCtx, err = sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			err = p.StartFreeTrial(userCtx)
			require.Error(t, err)
			require.True(t, console.ErrUnauthorized.Has(err))
		})
	})
}

func TestMigrateProjectPricing(t *testing.T) {
	var (
		legacyPlacement0  = storj.DefaultPlacement
		legacyPlacement12 = storj.PlacementConstraint(12)
		legacyPlacements  = []storj.PlacementConstraint{legacyPlacement0, legacyPlacement12}

		newPlacement10 = storj.PlacementConstraint(10)
		newPlacement20 = storj.PlacementConstraint(20)
		newPlacement30 = storj.PlacementConstraint(30)
		newPlacements  = []storj.PlacementConstraint{newPlacement10, newPlacement20, newPlacement30}

		// Legacy products (old billing before migration).
		legacyProduct100 = int32(100)
		legacyProduct200 = int32(200)

		// New products (used after migration).
		product1 = int32(1)
		product2 = int32(2)
		product3 = int32(3)
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "legacy-global");12:annotation("location", "legacy-us");10:annotation("location", "new-global");20:annotation("location", "new-us");30:annotation("location", "new-archive");`,
				}
				config.Console.Placement.AllowedPlacementIdsForNewProjects = newPlacements
				config.Console.LegacyPlacements = []string{"0", "12"}
				config.Entitlements.Enabled = true

				var mappingForMigration console.PlacementProductMappings
				err := mappingForMigration.Set(`{"0":1,"12":2}`)
				require.NoError(t, err)
				config.Console.LegacyPlacementProductMappingForMigration = mappingForMigration

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					int(newPlacement10): product1,
					int(newPlacement20): product2,
					int(newPlacement30): product3,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap

				price := paymentsconfig.ProjectUsagePrice{
					StorageTB: "4",
					EgressTB:  "7",
					Segment:   "0.0000088",
				}
				var productOverrides paymentsconfig.ProductPriceOverrides
				productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					product1:         {ProjectUsagePrice: price},
					product2:         {ProjectUsagePrice: price},
					product3:         {ProjectUsagePrice: price},
					legacyProduct100: {ProjectUsagePrice: price},
					legacyProduct200: {ProjectUsagePrice: price},
				})
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@example.com",
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		p, err := service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "legacy-project"})
		require.NoError(t, err)

		feats := entitlements.ProjectFeatures{
			NewBucketPlacements: legacyPlacements,
			PlacementProductMappings: entitlements.PlacementProductMappings{
				legacyPlacement0:  legacyProduct100,
				legacyPlacement12: legacyProduct200,
			},
		}
		featBytes, err := json.Marshal(feats)
		require.NoError(t, err)

		_, err = sat.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
			Scope:     entitlements.ConvertPublicIDToProjectScope(p.PublicID),
			Features:  featBytes,
			UpdatedAt: time.Now(),
		})
		require.NoError(t, err)

		entitlement, err := sat.API.Entitlements.Service.Projects().GetByPublicID(ctx, p.PublicID)
		require.NoError(t, err)
		require.Equal(t, legacyPlacements, entitlement.NewBucketPlacements)
		require.Equal(t, legacyProduct100, entitlement.PlacementProductMappings[legacyPlacement0])
		require.Equal(t, legacyProduct200, entitlement.PlacementProductMappings[legacyPlacement12])

		t.Run("successful migration as admin", func(t *testing.T) {
			err = service.MigrateProjectPricing(userCtx, p.PublicID)
			require.NoError(t, err)

			updatedEnt, err := sat.API.Entitlements.Service.Projects().GetByPublicID(ctx, p.PublicID)
			require.NoError(t, err)
			require.Equal(t, newPlacements, updatedEnt.NewBucketPlacements)
			require.NotNil(t, updatedEnt.PlacementProductMappings)

			// Verify legacy placements are mapped to new products.
			require.Equal(t, product1, updatedEnt.PlacementProductMappings[legacyPlacement0])
			require.Equal(t, product2, updatedEnt.PlacementProductMappings[legacyPlacement12])

			// Verify new placements are still mapped to new products.
			require.Equal(t, product1, updatedEnt.PlacementProductMappings[newPlacement10])
			require.Equal(t, product2, updatedEnt.PlacementProductMappings[newPlacement20])
			require.Equal(t, product3, updatedEnt.PlacementProductMappings[newPlacement30])
		})

		t.Run("cannot migrate non-legacy project", func(t *testing.T) {
			err = service.MigrateProjectPricing(userCtx, p.PublicID)
			require.Error(t, err)
			require.True(t, console.ErrConflict.Has(err))
			require.Contains(t, err.Error(), "classic projects")
		})

		t.Run("member without admin role cannot migrate", func(t *testing.T) {
			legacyProject, err := service.CreateProject(userCtx, console.UpsertProjectInfo{Name: "legacy-project-2"})
			require.NoError(t, err)

			feats := entitlements.ProjectFeatures{
				NewBucketPlacements: legacyPlacements,
				PlacementProductMappings: entitlements.PlacementProductMappings{
					legacyPlacement0:  legacyProduct100,
					legacyPlacement12: legacyProduct200,
				},
			}
			featBytes, err := json.Marshal(feats)
			require.NoError(t, err)

			_, err = sat.DB.Console().Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
				Scope:     entitlements.ConvertPublicIDToProjectScope(legacyProject.PublicID),
				Features:  featBytes,
				UpdatedAt: time.Now(),
			})
			require.NoError(t, err)

			memberUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Member User",
				Email:    "member@example.com",
			}, 1)
			require.NoError(t, err)

			_, err = sat.DB.Console().ProjectMembers().Insert(ctx, memberUser.ID, legacyProject.ID, console.RoleMember)
			require.NoError(t, err)

			memberCtx, err := sat.UserContext(ctx, memberUser.ID)
			require.NoError(t, err)

			err = service.MigrateProjectPricing(memberCtx, legacyProject.PublicID)
			require.Error(t, err)
			require.True(t, console.ErrForbidden.Has(err))
			require.Contains(t, err.Error(), "only project owner or admin")
		})
	})
}

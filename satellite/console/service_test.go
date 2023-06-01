// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/currency"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	"storj.io/storj/satellite/payments/stripe"
)

func TestService(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 2,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Payments.StripeCoinPayments.StripeFreeTierCouponID = stripe.MockCouponID1
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			service := sat.API.Console.Service

			up1Pro1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
			require.NoError(t, err)
			up2Pro1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[1].Projects[0].ID)
			require.NoError(t, err)

			up2User, err := sat.API.DB.Console().Users().Get(ctx, up2Pro1.OwnerID)
			require.NoError(t, err)

			require.NotEqual(t, up1Pro1.ID, up2Pro1.ID)
			require.NotEqual(t, up1Pro1.OwnerID, up2Pro1.OwnerID)

			userCtx1, err := sat.UserContext(ctx, up1Pro1.OwnerID)
			require.NoError(t, err)

			userCtx2, err := sat.UserContext(ctx, up2Pro1.OwnerID)
			require.NoError(t, err)

			t.Run("GetProject", func(t *testing.T) {
				// Getting own project details should work
				project, err := service.GetProject(userCtx1, up1Pro1.ID)
				require.NoError(t, err)
				require.Equal(t, up1Pro1.ID, project.ID)

				// Getting someone else project details should not work
				project, err = service.GetProject(userCtx1, up2Pro1.ID)
				require.Error(t, err)
				require.Nil(t, project)
			})

			t.Run("GetSalt", func(t *testing.T) {
				// Getting project salt as a member should work
				salt, err := service.GetSalt(userCtx1, up1Pro1.ID)
				require.NoError(t, err)
				require.NotNil(t, salt)

				// Getting project salt with publicID should work
				salt1, err := service.GetSalt(userCtx1, up1Pro1.PublicID)
				require.NoError(t, err)
				require.NotNil(t, salt1)

				// project.PublicID salt should be the same as project.ID salt
				require.Equal(t, salt, salt1)

				// Getting project salt as a non-member should not work
				salt, err = service.GetSalt(userCtx1, up2Pro1.ID)
				require.Error(t, err)
				require.Nil(t, salt)
			})

			t.Run("AddCreditCard fails when payments.CreditCards.Add returns error", func(t *testing.T) {
				// user should be in free tier
				user, err := service.GetUser(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.False(t, user.PaidTier)
				// get context
				userCtx1, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				// stripecoinpayments.TestPaymentMethodsAttachFailure triggers the underlying mock stripe client to return an error
				// when attaching a payment method to a customer.
				_, err = service.Payments().AddCreditCard(userCtx1, stripe.TestPaymentMethodsAttachFailure)
				require.Error(t, err)

				// user still in free tier
				user, err = service.GetUser(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.False(t, user.PaidTier)

				cards, err := service.Payments().ListCreditCards(userCtx1)
				require.NoError(t, err)
				require.Len(t, cards, 0)
			})

			t.Run("AddCreditCard", func(t *testing.T) {
				// user should be in free tier
				user, err := service.GetUser(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.False(t, user.PaidTier)
				// get context
				userCtx1, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)
				// add a credit card to put the user in the paid tier
				card, err := service.Payments().AddCreditCard(userCtx1, "test-cc-token")
				require.NoError(t, err)
				require.NotEmpty(t, card)
				// user should be in paid tier
				user, err = service.GetUser(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.True(t, user.PaidTier)

				cards, err := service.Payments().ListCreditCards(userCtx1)
				require.NoError(t, err)
				require.Len(t, cards, 1)
			})

			t.Run("CreateProject", func(t *testing.T) {
				// Creating a project with a previously used name should fail
				createdProject, err := service.CreateProject(userCtx1, console.ProjectInfo{
					Name: up1Pro1.Name,
				})
				require.Error(t, err)
				require.Nil(t, createdProject)
			})

			t.Run("UpdateProject", func(t *testing.T) {
				updatedName := "newName"
				updatedDescription := "newDescription"
				updatedStorageLimit := memory.Size(100)
				updatedBandwidthLimit := memory.Size(100)

				user, err := service.GetUser(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)

				userCtx1, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)

				// Updating own project should work
				updatedProject, err := service.UpdateProject(userCtx1, up1Pro1.ID, console.ProjectInfo{
					Name:           updatedName,
					Description:    updatedDescription,
					StorageLimit:   updatedStorageLimit,
					BandwidthLimit: updatedBandwidthLimit,
				})
				require.NoError(t, err)
				require.NotEqual(t, up1Pro1.Name, updatedProject.Name)
				require.Equal(t, updatedName, updatedProject.Name)
				require.NotEqual(t, up1Pro1.Description, updatedProject.Description)
				require.Equal(t, updatedDescription, updatedProject.Description)
				require.NotEqual(t, *up1Pro1.StorageLimit, *updatedProject.StorageLimit)
				require.Equal(t, updatedStorageLimit, *updatedProject.StorageLimit)
				require.NotEqual(t, *up1Pro1.BandwidthLimit, *updatedProject.BandwidthLimit)
				require.Equal(t, updatedBandwidthLimit, *updatedProject.BandwidthLimit)
				require.Equal(t, updatedStorageLimit, *updatedProject.UserSpecifiedStorageLimit)
				require.Equal(t, updatedBandwidthLimit, *updatedProject.UserSpecifiedBandwidthLimit)

				// Updating someone else project details should not work
				updatedProject, err = service.UpdateProject(userCtx1, up2Pro1.ID, console.ProjectInfo{
					Name:           "newName",
					Description:    "TestUpdate",
					StorageLimit:   memory.Size(100),
					BandwidthLimit: memory.Size(100),
				})
				require.Error(t, err)
				require.Nil(t, updatedProject)

				// attempting to update a project with bandwidth or storage limits set to 0 should fail
				size0 := new(memory.Size)
				*size0 = 0
				size100 := new(memory.Size)
				*size100 = memory.Size(100)

				up1Pro1.StorageLimit = size0
				err = sat.DB.Console().Projects().Update(ctx, up1Pro1)
				require.NoError(t, err)

				updateInfo := console.ProjectInfo{
					Name:           "a b c",
					Description:    "1 2 3",
					StorageLimit:   memory.Size(123),
					BandwidthLimit: memory.Size(123),
				}
				updatedProject, err = service.UpdateProject(userCtx1, up1Pro1.ID, updateInfo)
				require.Error(t, err)
				require.Nil(t, updatedProject)

				up1Pro1.StorageLimit = size100
				up1Pro1.BandwidthLimit = size0

				err = sat.DB.Console().Projects().Update(ctx, up1Pro1)
				require.NoError(t, err)

				updatedProject, err = service.UpdateProject(userCtx1, up1Pro1.ID, updateInfo)
				require.Error(t, err)
				require.Nil(t, updatedProject)

				up1Pro1.StorageLimit = size100
				up1Pro1.BandwidthLimit = size100
				err = sat.DB.Console().Projects().Update(ctx, up1Pro1)
				require.NoError(t, err)

				updatedProject, err = service.UpdateProject(userCtx1, up1Pro1.ID, updateInfo)
				require.NoError(t, err)
				require.Equal(t, updateInfo.Name, updatedProject.Name)
				require.Equal(t, updateInfo.Description, updatedProject.Description)
				require.NotNil(t, updatedProject.StorageLimit)
				require.NotNil(t, updatedProject.BandwidthLimit)
				require.Equal(t, updateInfo.StorageLimit, *updatedProject.StorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, *updatedProject.BandwidthLimit)

				project, err := service.GetProject(userCtx1, up1Pro1.ID)
				require.NoError(t, err)
				require.Equal(t, updateInfo.StorageLimit, *project.StorageLimit)
				require.Equal(t, updateInfo.BandwidthLimit, *project.BandwidthLimit)

				// attempting to update a project with a previously used name should fail
				updatedProject, err = service.UpdateProject(userCtx1, up2Pro1.ID, console.ProjectInfo{
					Name: up1Pro1.Name,
				})
				require.Error(t, err)
				require.Nil(t, updatedProject)
			})

			t.Run("AddProjectMembers", func(t *testing.T) {
				// Adding members to own project should work
				addedUsers, err := service.AddProjectMembers(userCtx1, up1Pro1.ID, []string{up2User.Email})
				require.NoError(t, err)
				require.Len(t, addedUsers, 1)
				require.Contains(t, addedUsers, up2User)

				// Adding members to someone else project should not work
				addedUsers, err = service.AddProjectMembers(userCtx1, up2Pro1.ID, []string{up2User.Email})
				require.Error(t, err)
				require.Nil(t, addedUsers)
			})

			t.Run("GetProjectMembers", func(t *testing.T) {
				// Getting the project members of an own project that one is a part of should work
				userPage, err := service.GetProjectMembers(userCtx1, up1Pro1.ID, console.ProjectMembersCursor{Page: 1, Limit: 10})
				require.NoError(t, err)
				require.Len(t, userPage.ProjectMembers, 2)

				// Getting the project members of a foreign project that one is a part of should work
				userPage, err = service.GetProjectMembers(userCtx2, up1Pro1.ID, console.ProjectMembersCursor{Page: 1, Limit: 10})
				require.NoError(t, err)
				require.Len(t, userPage.ProjectMembers, 2)

				// Getting the project members of a foreign project that one is not a part of should not work
				userPage, err = service.GetProjectMembers(userCtx1, up2Pro1.ID, console.ProjectMembersCursor{Page: 1, Limit: 10})
				require.Error(t, err)
				require.Nil(t, userPage)
			})

			t.Run("DeleteProjectMembers", func(t *testing.T) {
				// Deleting project members of an own project should work
				err := service.DeleteProjectMembers(userCtx1, up1Pro1.ID, []string{up2User.Email})
				require.NoError(t, err)

				// Deleting Project members of someone else project should not work
				err = service.DeleteProjectMembers(userCtx1, up2Pro1.ID, []string{up2User.Email})
				require.Error(t, err)
			})

			t.Run("DeleteProject", func(t *testing.T) {
				// Deleting the own project should not work before deleting the API-Key
				err := service.DeleteProject(userCtx1, up1Pro1.ID)
				require.Error(t, err)

				keys, err := service.GetAPIKeys(userCtx1, up1Pro1.ID, console.APIKeyCursor{Page: 1, Limit: 10})
				require.NoError(t, err)
				require.Len(t, keys.APIKeys, 1)

				err = service.DeleteAPIKeys(userCtx1, []uuid.UUID{keys.APIKeys[0].ID})
				require.NoError(t, err)

				// Deleting the own project should now work
				err = service.DeleteProject(userCtx1, up1Pro1.ID)
				require.NoError(t, err)

				// Deleting someone else project should not work
				err = service.DeleteProject(userCtx1, up2Pro1.ID)
				require.Error(t, err)

				err = planet.Uplinks[1].CreateBucket(ctx, sat, "testbucket")
				require.NoError(t, err)

				// deleting a project with a bucket should fail
				err = service.DeleteProject(userCtx2, up2Pro1.ID)
				require.Error(t, err)
				require.Equal(t, "console service: project usage: some buckets still exist", err.Error())
			})

			t.Run("GetProjectUsageLimits", func(t *testing.T) {
				bandwidthLimit := sat.Config.Console.UsageLimits.Bandwidth.Free
				storageLimit := sat.Config.Console.UsageLimits.Storage.Free

				limits1, err := service.GetProjectUsageLimits(userCtx2, up2Pro1.ID)
				require.NoError(t, err)
				require.NotNil(t, limits1)

				// Get usage limits with publicID
				limits2, err := service.GetProjectUsageLimits(userCtx2, up2Pro1.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)

				// limits gotten by ID and publicID should be the same
				require.Equal(t, storageLimit.Int64(), limits1.StorageLimit)
				require.Equal(t, bandwidthLimit.Int64(), limits1.BandwidthLimit)
				require.Equal(t, storageLimit.Int64(), limits2.StorageLimit)
				require.Equal(t, bandwidthLimit.Int64(), limits2.BandwidthLimit)

				// update project's limits
				updatedStorageLimit := memory.Size(100) + memory.TB
				updatedBandwidthLimit := memory.Size(100) + memory.TB
				up2Pro1.StorageLimit = new(memory.Size)
				*up2Pro1.StorageLimit = updatedStorageLimit
				up2Pro1.BandwidthLimit = new(memory.Size)
				*up2Pro1.BandwidthLimit = updatedBandwidthLimit
				err = sat.DB.Console().Projects().Update(ctx, up2Pro1)
				require.NoError(t, err)

				limits1, err = service.GetProjectUsageLimits(userCtx2, up2Pro1.ID)
				require.NoError(t, err)
				require.NotNil(t, limits1)

				// Get usage limits with publicID
				limits2, err = service.GetProjectUsageLimits(userCtx2, up2Pro1.PublicID)
				require.NoError(t, err)
				require.NotNil(t, limits2)

				// limits gotten by ID and publicID should be the same
				require.Equal(t, updatedStorageLimit.Int64(), limits1.StorageLimit)
				require.Equal(t, updatedBandwidthLimit.Int64(), limits1.BandwidthLimit)
				require.Equal(t, updatedStorageLimit.Int64(), limits2.StorageLimit)
				require.Equal(t, updatedBandwidthLimit.Int64(), limits2.BandwidthLimit)
			})

			t.Run("ChangeEmail", func(t *testing.T) {
				const newEmail = "newEmail@example.com"

				err = service.ChangeEmail(userCtx2, newEmail)
				require.NoError(t, err)

				user, _, err := service.GetUserByEmailWithUnverified(userCtx2, newEmail)
				require.NoError(t, err)
				require.Equal(t, newEmail, user.Email)

				err = service.ChangeEmail(userCtx2, newEmail)
				require.Error(t, err)
			})

			t.Run("GetAllBucketNames", func(t *testing.T) {
				bucket1 := buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      "testBucket1",
					ProjectID: up2Pro1.ID,
				}

				bucket2 := buckets.Bucket{
					ID:        testrand.UUID(),
					Name:      "testBucket2",
					ProjectID: up2Pro1.ID,
				}

				_, err := sat.API.Buckets.Service.CreateBucket(userCtx2, bucket1)
				require.NoError(t, err)

				_, err = sat.API.Buckets.Service.CreateBucket(userCtx2, bucket2)
				require.NoError(t, err)

				bucketNames, err := service.GetAllBucketNames(userCtx2, up2Pro1.ID)
				require.NoError(t, err)
				require.Equal(t, bucket1.Name, bucketNames[0])
				require.Equal(t, bucket2.Name, bucketNames[1])

				bucketNames, err = service.GetAllBucketNames(userCtx2, up2Pro1.PublicID)
				require.NoError(t, err)
				require.Equal(t, bucket1.Name, bucketNames[0])
				require.Equal(t, bucket2.Name, bucketNames[1])

				// Getting someone else buckets should not work
				bucketsForUnauthorizedUser, err := service.GetAllBucketNames(userCtx1, up2Pro1.ID)
				require.Error(t, err)
				require.Nil(t, bucketsForUnauthorizedUser)
			})

			t.Run("DeleteAPIKeyByNameAndProjectID", func(t *testing.T) {
				secret, err := macaroon.NewSecret()
				require.NoError(t, err)

				key, err := macaroon.NewAPIKey(secret)
				require.NoError(t, err)

				apikey := console.APIKeyInfo{
					Name:      "test",
					ProjectID: up2Pro1.ID,
					Secret:    secret,
				}

				createdKey, err := sat.DB.Console().APIKeys().Create(ctx, key.Head(), apikey)
				require.NoError(t, err)

				info, err := sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.NoError(t, err)
				require.NotNil(t, info)

				// Deleting someone else api keys should not work
				err = service.DeleteAPIKeyByNameAndProjectID(userCtx1, apikey.Name, up2Pro1.ID)
				require.Error(t, err)

				err = service.DeleteAPIKeyByNameAndProjectID(userCtx2, apikey.Name, up2Pro1.ID)
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
				err = service.DeleteAPIKeyByNameAndProjectID(userCtx2, apikey.Name, up2Pro1.PublicID)
				require.NoError(t, err)

				info, err = sat.DB.Console().APIKeys().Get(ctx, createdKey.ID)
				require.Error(t, err)
				require.Nil(t, info)
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

				coupon, err = sat.API.Payments.Accounts.Coupons().GetByUserID(ctx, up1Pro1.OwnerID)
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

				coupon, err = sat.API.Payments.Accounts.Coupons().GetByUserID(ctx, up2Pro1.OwnerID)
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
					dbPackagePlan, dbPurchaseTime, err := sat.DB.StripeCoinPayments().Customers().GetPackageInfo(ctx, up1Pro1.OwnerID)
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
				btxs, err := sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.Zero(t, len(btxs))
			})
			t.Run("ApplyCredit", func(t *testing.T) {
				amount := int64(1000)
				desc := "test"
				require.NoError(t, service.Payments().ApplyCredit(userCtx1, 1000, desc))
				btxs, err := sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.Len(t, btxs, 1)
				require.Equal(t, amount, btxs[0].Amount)
				require.Equal(t, desc, btxs[0].Description)

				// test same description results in no new credit
				require.NoError(t, service.Payments().ApplyCredit(userCtx1, 1000, desc))
				btxs, err = sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.Len(t, btxs, 1)

				// test different description results in new credit
				require.NoError(t, service.Payments().ApplyCredit(userCtx1, 1000, "new desc"))
				btxs, err = sat.API.Payments.Accounts.Balances().ListTransactions(ctx, up1Pro1.OwnerID)
				require.NoError(t, err)
				require.Len(t, btxs, 2)
			})
			t.Run("ApplyCredit fails with unknown user", func(t *testing.T) {
				require.Error(t, service.Payments().ApplyCredit(ctx, 1000, "test"))
			})
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
		require.False(t, user.PaidTier)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		// add a credit card to the user
		_, err = service.Payments().AddCreditCard(userCtx, "test-cc-token")
		require.NoError(t, err)

		// expect user to be in paid tier
		user, err = service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, user.PaidTier)
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
		proj2, err := service.CreateProject(userCtx, console.ProjectInfo{Name: "Project 2"})
		require.NoError(t, err)
		require.Equal(t, usageConfig.Storage.Paid, *proj2.StorageLimit)
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
		_, err = service.UpdateProject(userCtx1, projectID, console.ProjectInfo{
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
		_, err = service.UpdateProject(userCtx1, projectID, console.ProjectInfo{
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

		var key string
		t.Run("ResetMFASecretKey", func(t *testing.T) {
			key, err = service.ResetMFASecretKey(userCtx)
			require.NoError(t, err)

			_, user := updateContext()
			require.NotEmpty(t, user.MFASecretKey)
		})

		t.Run("EnableUserMFABadPasscode", func(t *testing.T) {
			// Expect MFA-enabling attempt to be rejected when providing stale passcode.
			badCode, err := console.NewMFAPasscode(key, time.Time{}.Add(time.Hour))
			require.NoError(t, err)

			err = service.EnableUserMFA(userCtx, badCode, time.Time{})
			require.True(t, console.ErrValidation.Has(err))

			userCtx, _ = updateContext()
			_, err = service.ResetMFARecoveryCodes(userCtx)
			require.True(t, console.ErrUnauthorized.Has(err))

			_, user = updateContext()
			require.False(t, user.MFAEnabled)
		})

		t.Run("EnableUserMFAGoodPasscode", func(t *testing.T) {
			// Expect MFA-enabling attempt to succeed when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.EnableUserMFA(userCtx, goodCode, time.Time{})
			require.NoError(t, err)

			_, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.Equal(t, user.MFASecretKey, key)
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
			_, err = service.ResetMFARecoveryCodes(userCtx)
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
			_, err = service.ResetMFARecoveryCodes(userCtx)
			require.NoError(t, err)
		})

		t.Run("DisableUserMFABadPasscode", func(t *testing.T) {
			// Expect MFA-disabling attempt to fail when providing valid passcode.
			badCode, err := console.NewMFAPasscode(key, time.Time{}.Add(time.Hour))
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.DisableUserMFA(userCtx, badCode, time.Time{}, "")
			require.True(t, console.ErrValidation.Has(err))

			_, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.NotEmpty(t, user.MFASecretKey)
			require.NotEmpty(t, user.MFARecoveryCodes)
		})

		t.Run("DisableUserMFAConflict", func(t *testing.T) {
			// Expect MFA-disabling attempt to fail when providing both recovery code and passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			userCtx, user = updateContext()
			err = service.DisableUserMFA(userCtx, goodCode, time.Time{}, user.MFARecoveryCodes[0])
			require.True(t, console.ErrMFAConflict.Has(err))

			_, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.NotEmpty(t, user.MFASecretKey)
			require.NotEmpty(t, user.MFARecoveryCodes)
		})

		t.Run("DisableUserMFAGoodPasscode", func(t *testing.T) {
			// Expect MFA-disabling attempt to succeed when providing valid passcode.
			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.DisableUserMFA(userCtx, goodCode, time.Time{}, "")
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

			goodCode, err := console.NewMFAPasscode(key, time.Time{})
			require.NoError(t, err)

			userCtx, _ = updateContext()
			err = service.EnableUserMFA(userCtx, goodCode, time.Time{})
			require.NoError(t, err)

			userCtx, _ = updateContext()
			_, err = service.ResetMFARecoveryCodes(userCtx)
			require.NoError(t, err)

			userCtx, user = updateContext()
			require.True(t, user.MFAEnabled)
			require.NotEmpty(t, user.MFASecretKey)
			require.NotEmpty(t, user.MFARecoveryCodes)

			// Disable MFA
			err = service.DisableUserMFA(userCtx, "", time.Time{}, user.MFARecoveryCodes[0])
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
		err = service.ResetPassword(ctx, token.Secret.String(), newPass, "", "", token.CreatedAt.Add(sat.Config.ConsoleAuth.TokenExpirationTime).Add(time.Second))
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
	})
}

func TestChangePassword(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		upl := planet.Uplinks[0]
		newPass := "newPass123!"

		user, err := sat.DB.Console().Users().GetByEmail(ctx, upl.User[sat.ID()].Email)
		require.NoError(t, err)
		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		// generate a password recovery token to test that changing password invalidates it
		passwordRecoveryToken, err := sat.API.Console.Service.GeneratePasswordRecoveryToken(userCtx, user.ID)
		require.NoError(t, err)

		require.NoError(t, sat.API.Console.Service.ChangePassword(userCtx, upl.User[sat.ID()].Password, newPass))
		user, err = sat.DB.Console().Users().Get(ctx, user.ID)
		require.NoError(t, err)
		require.NoError(t, bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(newPass)))

		err = sat.API.Console.Service.ResetPassword(userCtx, passwordRecoveryToken, "aDifferentPassword123!", "", "", time.Now())
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
		token1, err := srv.GenerateSessionToken(userCtx, user.ID, user.Email, "", "")
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
		token2, err := srv.GenerateSessionToken(userCtx, user.ID, user.Email, "", "")
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
		token3, err := srv.GenerateSessionToken(userCtx, user.ID, user.Email, "", "")
		require.NoError(t, err)
		token3Duration := token3.ExpiresAt.Sub(now)
		require.Less(t, token3Duration, token1Duration)
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
		token, err := srv.GenerateSessionToken(userCtx, user.ID, user.Email, "", "")
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

		newUser, err := userDB.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "newuser@example.com",
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

		onboardingBool := true
		onboardingStep := "Overview"
		sessionDur := time.Duration(rand.Int63()).Round(time.Minute)
		sessionDurPtr := &sessionDur
		settings, err = srv.SetUserSettings(userCtx, console.UpsertUserSettingsRequest{
			SessionDuration: &sessionDurPtr,
			OnboardingStart: &onboardingBool,
			OnboardingEnd:   &onboardingBool,
			OnboardingStep:  &onboardingStep,
		})
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)

		settings, err = userDB.GetSettings(userCtx, newUser.ID)
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)

		// passing nil should not override existing values
		settings, err = srv.SetUserSettings(userCtx, console.UpsertUserSettingsRequest{
			SessionDuration: nil,
			OnboardingStart: nil,
			OnboardingEnd:   nil,
			OnboardingStep:  nil,
		})
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)

		settings, err = userDB.GetSettings(userCtx, newUser.ID)
		require.NoError(t, err)
		require.Equal(t, onboardingBool, settings.OnboardingStart)
		require.Equal(t, onboardingBool, settings.OnboardingEnd)
		require.Equal(t, &onboardingStep, settings.OnboardingStep)
		require.Equal(t, sessionDurPtr, settings.SessionDuration)
	})
}

func TestRESTKeys(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		proj1, err := sat.API.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		user, err := service.GetUser(ctx, proj1.OwnerID)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		now := time.Now()
		expires := 5 * time.Hour
		apiKey, expiresAt, err := service.CreateRESTKey(userCtx, expires)
		require.NoError(t, err)
		require.NotEmpty(t, apiKey)
		require.True(t, expiresAt.After(now))
		require.True(t, expiresAt.Before(now.Add(expires+time.Hour)))

		// test revocation
		require.NoError(t, service.RevokeRESTKey(userCtx, apiKey))

		// test revoke non existent key
		nonexistent := testrand.UUID()
		err = service.RevokeRESTKey(userCtx, nonexistent.String())
		require.Error(t, err)
	})
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
		err = service.UpdateUsersFailedLoginState(userCtx, lockedUser)
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

		_, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		sessionID, err := uuid.FromBytes(tokenInfo.Token.Payload)
		require.NoError(t, err)

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID)
		require.NoError(t, err)

		// Session should be removed from DB after it has expired
		_, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now().Add(2*time.Hour))
		require.True(t, console.ErrTokenExpiration.Has(err))

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID)
		require.ErrorIs(t, sql.ErrNoRows, err)
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

		_, err = service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		require.NoError(t, err)

		sessionID, err := uuid.FromBytes(tokenInfo.Token.Payload)
		require.NoError(t, err)

		_, err = sat.DB.Console().WebappSessions().GetBySessionID(ctx, sessionID)
		require.NoError(t, err)

		// Session2 should be added to DB after token request
		tokenInfo2, err := service.Token(ctx, console.AuthUser{Email: user.Email, Password: user.FullName})
		require.NoError(t, err)

		_, err = service.TokenAuth(ctx, tokenInfo2.Token, time.Now())
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
				ID:        coinpayments.TransactionID(fmt.Sprintf("%d", i)),
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
			cachedPayments = append(cachedPayments, storjscan.CachedPayment{
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
			return cachedPayments[i].BlockNumber > cachedPayments[j].BlockNumber
		})
		sort.Slice(transactions, func(i, j int) bool {
			return transactions[i].CreatedAt.After(transactions[j].CreatedAt)
		})

		var expected []console.PaymentInfo
		for _, pmnt := range cachedPayments {
			expected = append(expected, console.PaymentInfo{
				ID:        fmt.Sprintf("%s#%d", pmnt.Transaction.Hex(), pmnt.LogIndex),
				Type:      "storjscan",
				Wallet:    pmnt.To.Hex(),
				Amount:    pmnt.USDValue,
				Status:    string(pmnt.Status),
				Link:      console.EtherscanURL(pmnt.Transaction.Hex()),
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
				ID:        fmt.Sprintf("%s#%d", meta.ReferenceID, meta.LogIndex),
				Type:      txn.Source,
				Wallet:    meta.Wallet,
				Amount:    txn.Amount,
				Status:    string(txn.Status),
				Link:      console.EtherscanURL(meta.ReferenceID),
				Timestamp: txn.Timestamp,
			})
		}

		walletPayments, err := sat.API.Console.Service.Payments().WalletPayments(reqCtx)
		require.NoError(t, err)
		require.Equal(t, expected, walletPayments.Payments)
	})
}

func TestPaymentsPurchase(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		p := sat.API.Console.Service.Payments()
		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		testDesc := "testDescription"
		testPaymentMethod := "testPaymentMethod"

		tests := []struct {
			name          string
			purchaseDesc  string
			paymentMethod string
			shouldErr     bool
			ctx           context.Context
		}{
			{
				"Purchase returns error with unknown user",
				testDesc,
				testPaymentMethod,
				true,
				ctx,
			},
			{
				"Purchase returns error when underlying payments.Invoices.New returns error",
				stripe.MockInvoicesNewFailure,
				testPaymentMethod,
				true,
				userCtx,
			},
			{
				"Purchase returns error when underlying payments.Invoices.Pay returns error",
				testDesc,
				stripe.MockInvoicesPayFailure,
				true,
				userCtx,
			},
			{
				"Purchase success",
				testDesc,
				testPaymentMethod,
				false,
				userCtx,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := p.Purchase(tt.ctx, 1000, tt.purchaseDesc, tt.paymentMethod)
				if tt.shouldErr {
					require.NotNil(t, err)
				} else {
					require.Nil(t, err)
				}
			})
		}

	})
}

func TestPaymentsPurchasePreexistingInvoice(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		p := sat.API.Console.Service.Payments()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@mail.test",
		}, 1)
		require.NoError(t, err)

		userCtx, err := sat.UserContext(ctx, user.ID)
		require.NoError(t, err)

		draftInvDesc := "testDraftDescription"
		testPaymentMethod := "testPaymentMethod"

		invs, err := sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 0)

		// test purchase with draft invoice
		inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(ctx, user.ID, 1000, draftInvDesc)
		require.NoError(t, err)
		require.Equal(t, payments.InvoiceStatusDraft, inv.Status)

		draftInv := inv.ID

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
		require.Equal(t, draftInv, invs[0].ID)

		require.NoError(t, p.Purchase(userCtx, 1000, draftInvDesc, stripe.MockInvoicesPaySuccess))

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 1)
		require.NotEqual(t, draftInv, invs[0].ID)
		require.Equal(t, payments.InvoiceStatusPaid, invs[0].Status)

		// test purchase with open invoice
		openInvDesc := "testOpenDescription"
		inv, err = sat.API.Payments.StripeService.Accounts().Invoices().Create(ctx, user.ID, 1000, openInvDesc)
		require.NoError(t, err)

		openInv := inv.ID

		// attempting to pay a draft invoice changes it to open if payment fails
		_, err = sat.API.Payments.StripeService.Accounts().Invoices().Pay(ctx, inv.ID, stripe.MockInvoicesPayFailure)
		require.Error(t, err)

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 2)
		var foundInv bool
		for _, inv := range invs {
			if inv.ID == openInv {
				foundInv = true
				require.Equal(t, payments.InvoiceStatusOpen, inv.Status)
			}
		}
		require.True(t, foundInv)

		require.NoError(t, p.Purchase(userCtx, 1000, openInvDesc, stripe.MockInvoicesPaySuccess))

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 2)
		foundInv = false
		for _, inv := range invs {
			if inv.ID == openInv {
				foundInv = true
				require.Equal(t, payments.InvoiceStatusPaid, inv.Status)
			}
		}
		require.True(t, foundInv)

		// purchase with paid invoice skips creating and or paying invoice
		require.NoError(t, p.Purchase(userCtx, 1000, openInvDesc, testPaymentMethod))

		invs, err = sat.API.Payments.StripeService.Accounts().Invoices().List(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, invs, 2)
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

				info := console.ProjectInfo{
					Name:           updatedName,
					Description:    updatedDescription,
					StorageLimit:   updatedStorageLimit,
					BandwidthLimit: updatedBandwidthLimit,
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
			require.NoError(t, tt.uplink.CreateBucket(tt.ctx, sat, bucket))
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
			p, err := s.CreateProject(tt.ctx, console.ProjectInfo{
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

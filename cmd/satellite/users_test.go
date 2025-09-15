// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"go.uber.org/zap/zaptest"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	storjstripe "storj.io/storj/satellite/payments/stripe"
	"storj.io/uplink"
)

func TestDeleteObjects(t *testing.T) {
	tcases := []struct {
		desc          string
		uncoordinated bool
	}{
		{"coordinated", false},
		{"uncoordinated", true},
	}
	for _, tcase := range tcases {
		t.Run(tcase.desc, func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				Reconfigure: testplanet.Reconfigure{
					Satellite: testplanet.Combine(
						testplanet.ReconfigureRS(2, 2, 4, 4),
						testplanet.MaxSegmentSize(13*memory.KiB),
					),
				},
				UplinkCount: 6, SatelliteCount: 1, StorageNodeCount: 4,
			}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
				sat := planet.Satellites[0]
				uplinks := planet.Uplinks
				require.Len(t, uplinks, 6) // The test is based on 6 uplinks

				bucketsObjects := map[string]map[string][]byte{
					"bucket1": {
						"single-segment-object":        testrand.Bytes(10 * memory.KiB),
						"multi-segment-object":         testrand.Bytes(50 * memory.KiB),
						"remote-segment-inline-object": testrand.Bytes(1 * memory.KiB),
					},
					"bucket2": {
						"multi-segment-object": testrand.Bytes(100 * memory.KiB),
					},
					"bucket3": {},
				}

				// 1st Uplink has a project with all the buckets.
				for bucketName, objects := range bucketsObjects {
					for objectName, bytes := range objects {
						require.NoError(t, uplinks[0].Upload(ctx, sat, bucketName, objectName, bytes))
					}
				}

				// 2nd Uplink has a project with one bucket with one object.
				require.NoError(t, uplinks[1].Upload(
					ctx, sat, "my-bucket", "multi-segment-object", bucketsObjects["bucket2"]["multi-segment-object"]),
				)

				// 3rd Uplink has a project with one empty bucket.
				require.NoError(t, uplinks[2].TestingCreateBucket(ctx, sat, "empty-bucket"))

				// 4th Uplink has an empty project.
				// 5th Uplink has project with some buckets and objects & a second project with a bucket with data.
				for bucketName, objects := range bucketsObjects {
					for objectName, bytes := range objects {
						require.NoError(t, uplinks[4].Upload(ctx, sat, bucketName, objectName, bytes))
					}
				}

				var ulkExtProject *uplink.Project
				{ // Create a new project associated with the 5th Uplink user and upload some objects.
					require.Len(t, uplinks[4].Projects, 1)
					owner := uplinks[4].Projects[0].Owner
					proj, err := sat.AddProject(ctx, owner.ID, "a second project")
					require.NoError(t, err)

					userCtx, err := sat.UserContext(ctx, owner.ID)
					require.NoError(t, err)
					_, apiKey, err := sat.API.Console.Service.CreateAPIKey(
						userCtx, proj.ID, "root", macaroon.APIKeyVersionObjectLock,
					)
					require.NoError(t, err)

					access, err := uplinks[4].Config.RequestAccessWithPassphrase(ctx, sat.URL(), apiKey.Serialize(), "")
					require.NoError(t, err)
					ulkExtProject, err = uplink.OpenProject(ctx, access)
					require.NoError(t, err)
					_, err = ulkExtProject.EnsureBucket(ctx, "my-test-bucket")
					require.NoError(t, err)
					upload, err := ulkExtProject.UploadObject(ctx, "my-test-bucket", "test-object", nil)
					require.NoError(t, err)
					_, err = upload.Write(testrand.Bytes(14 * memory.KiB))
					require.NoError(t, err)
					require.NoError(t, upload.Commit())
				}

				// 6th Uplink has a project with one bucket with one object, but the user's won't be set to
				// "pending deletion" status.
				require.NoError(t, uplinks[5].Upload(
					ctx, sat, "my-bucket", "my-object", bucketsObjects["bucket1"]["single-segment-object"]),
				)

				// Ensure the number of objects before the deletion.
				objects, err := sat.Metabase.DB.TestingAllObjects(ctx)
				require.NoError(t, err)
				require.Len(t, objects, 11)

				// Set the accounts in "pending deletion" status, except the 6th Uplink.
				for i := 0; i < len(uplinks)-1; i++ {
					pendingStatus := console.PendingDeletion
					require.NoError(t,
						sat.DB.Console().Users().Update(ctx, uplinks[i].Projects[0].Owner.ID, console.UpdateUserRequest{
							Status: &pendingStatus,
						}))
				}

				// Create a CSV with the users' emails to delete.
				var csvData io.Reader
				{
					emails := "email"
					for _, uplnk := range uplinks {
						emails += "\n" + uplnk.User[sat.ID()].Email
					}

					csvData = bytes.NewBufferString(emails)
				}

				// Delete all the data of the accounts.
				require.NoError(t, deleteObjects(
					ctx, zaptest.NewLogger(t), sat.DB, sat.Metabase.DB, 20, tcase.uncoordinated, csvData,
				))

				// Check that all the data was deleted.
				objects, err = sat.Metabase.DB.TestingAllObjects(ctx)
				require.NoError(t, err)
				require.Len(t, objects, 1) // The user of the 6th is not in "pending deletion" status.

				// check that there aren't buckets.
				for i := 0; i < len(uplinks)-1; i++ {
					buckets, err := uplinks[i].ListBuckets(ctx, sat)
					require.NoError(t, err)
					require.Len(t, buckets, 0)
				}

				ulkExtBuckets := ulkExtProject.ListBuckets(ctx, &uplink.ListBucketsOptions{})
				require.False(t, ulkExtBuckets.Next())

				{ // Verify that the 6th uplink has a its data, a bucket and an object.
					buckets, err := uplinks[5].ListBuckets(ctx, sat)
					require.NoError(t, err)
					require.Len(t, buckets, 1)

					objects, err := uplinks[5].ListObjects(ctx, sat, buckets[0].Name)
					require.NoError(t, err)
					require.Len(t, objects, 1)
				}
			})
		})
	}
}

func TestDeleteObjectsFromNonExistingBucket(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		require.NoError(t, planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "object", testrand.Bytes(100)))

		err := deleteNonExistingBucketObjects(ctx, zaptest.NewLogger(t), planet.Satellites[0].DB.Buckets(), planet.Satellites[0].Metabase.DB, planet.Uplinks[0].Projects[0].ID, "testbucket", 10)
		require.Error(t, err)

		err = planet.Satellites[0].DB.Buckets().DeleteBucket(ctx, []byte("testbucket"), planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		err = deleteNonExistingBucketObjects(ctx, zaptest.NewLogger(t), planet.Satellites[0].DB.Buckets(), planet.Satellites[0].Metabase.DB, planet.Uplinks[0].Projects[0].ID, "testbucket", 10)
		require.NoError(t, err)

		objects, err := planet.Satellites[0].Metabase.DB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 0)

		segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 0)
	})
}

func TestDeleteAccounts(t *testing.T) {
	// The test is based on 16 uplinks because it offers having several users with projects.
	// The test uses them to create the following scenario following the order of the uplinks. If
	// not specified, the user is as provided by testplanet with no extra API keys, isn't member of
	// any project, no data, no usage, no invoices, no pending invoice items.
	// - 1) user with status "pending deletion".
	// - 2) user with status "pending deletion", 5 API keys on its project, and paid tier.
	// - 3) user with status "pending deletion", and member of the project of the 4th uplink.
	// - 4) user with status "pending deletion", member of the project of the 3th uplink, and paid tier.
	// - 5) user with status "pending deletion", and member of the project of the 4rd and 6th uplink.
	// - 6) user with status "pending deletion" and 1 additional project.
	// - 7) user with status "pending deletion" and a bucket and object.
	// - 8) user with status "pending deletion", usage, and paid tier.
	// - 9) user with status "pending deletion" , invoice in "draft" status, and paid tier.
	// - 10) user with status "pending deletion", invoice in "open" status, and paid tier.
	// - 11) user with status "pending deletion", invoice in "paid" status, and paid tier.
	// - 12) user with status "pending deletion", invoice in "void" status, and paid tier.
	// - 13) user with status "pending deletion", invoice in "uncollectible" status, and paid tier.
	// - 14) user with status "pending deletion", pending invoice items, and paid tier.
	// - 15) user status "active".
	// - 16) user status "legal hold". NOTE: We don't check all the statuses, but we consider that
	//       the "active" status is an usual one, so we check the "legal hold" as a differentiation.
	const uplinkCount = 16

	testplanet.Run(t, testplanet.Config{
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 2, 4, 4),
				testplanet.MaxSegmentSize(13*memory.KiB),
			),
		},
		UplinkCount: uplinkCount, SatelliteCount: 1, StorageNodeCount: 4,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		uplinks := planet.Uplinks
		require.Len(t, uplinks, uplinkCount)

		{ // Add 4 API keys more to the 2nd uplink project.
			userCtx, err := sat.UserContext(ctx, uplinks[1].Projects[0].Owner.ID)
			require.NoError(t, err)
			for i := 1; i <= 3; i++ {
				_, _, err := sat.API.Console.Service.CreateAPIKey(
					userCtx, uplinks[1].Projects[0].ID, strconv.Itoa(i), macaroon.APIKeyVersionObjectLock,
				)
				require.NoError(t, err)
			}
		}

		{ // Add the 3rd uplink user to the 4th uplink project.
			userCtx, err := sat.UserContext(ctx, uplinks[3].Projects[0].Owner.ID)
			require.NoError(t, err)
			_, err = sat.API.Console.Service.AddProjectMembers(userCtx,
				uplinks[3].Projects[0].ID, []string{uplinks[2].User[sat.ID()].Email},
			)
			require.NoError(t, err)
		}

		{ // Add the 4th uplink user to the 3rd uplink project.
			userCtx, err := sat.UserContext(ctx, uplinks[2].Projects[0].Owner.ID)
			require.NoError(t, err)
			_, err = sat.API.Console.Service.AddProjectMembers(userCtx,
				uplinks[2].Projects[0].ID, []string{uplinks[3].User[sat.ID()].Email},
			)
			require.NoError(t, err)
		}

		{ // Add the 5th uplink user to the 4th and 6th uplink projects.
			userCtx, err := sat.UserContext(ctx, uplinks[3].Projects[0].Owner.ID)
			require.NoError(t, err)
			_, err = sat.API.Console.Service.AddProjectMembers(userCtx,
				uplinks[3].Projects[0].ID, []string{uplinks[4].User[sat.ID()].Email},
			)
			require.NoError(t, err)
			userCtx, err = sat.UserContext(ctx, uplinks[5].Projects[0].Owner.ID)
			require.NoError(t, err)
			_, err = sat.API.Console.Service.AddProjectMembers(userCtx,
				uplinks[5].Projects[0].ID, []string{uplinks[4].User[sat.ID()].Email},
			)
			require.NoError(t, err)
		}

		// Add a second project to the 6th Uplink user.
		var extraProj *console.Project
		{ // Create a new project associated with the 5th Uplink user and upload some objects.
			require.Len(t, uplinks[5].Projects, 1)

			var err error
			owner := uplinks[4].Projects[0].Owner
			extraProj, err = sat.AddProject(ctx, owner.ID, "a second project")
			require.NoError(t, err)

			// Create 3 API keys for the project.
			userCtx, err := sat.UserContext(ctx, owner.ID)
			require.NoError(t, err)
			for i := 1; i <= 3; i++ {
				_, _, err := sat.API.Console.Service.CreateAPIKey(
					userCtx, extraProj.ID, strconv.Itoa(i), macaroon.APIKeyVersionObjectLock,
				)
				require.NoError(t, err)
			}
		}

		// Upload an object to the 7th Uplink's project.
		require.NoError(t, uplinks[6].Upload(
			ctx, sat, "my-bucket", "my-object", testrand.Bytes(10*memory.KiB)),
		)

		{ // Create usage for the project of the 8th Uplink.
			since := time.Now()
			sat.Accounting.Tally.Loop.Pause()

			require.NoError(t, uplinks[7].Upload(
				ctx, sat, "my-bucket", "my-object", testrand.Bytes(10*memory.KiB),
			))

			_, err := uplinks[7].Download(ctx, sat, "my-bucket", "my-object")
			require.NoError(t, err)

			// sat.Accounting.Tally.Loop.TriggerWait()
			require.NoError(t, uplinks[7].DeleteObject(ctx, sat, "my-bucket", "my-object"))
			require.NoError(t, uplinks[7].DeleteBucket(ctx, sat, "my-bucket"))

			// Wait for the SNs endpoints to finish their work
			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

			// Ensure all nodes have sent up any orders for the time period we're calculating
			for _, sn := range planet.StorageNodes {
				sn.Storage2.Orders.SendOrders(ctx, since.Add(24*time.Hour))
			}

			sat.Accounting.Tally.Loop.TriggerWait()
			// flush rollups write cache
			sat.Orders.Chore.Loop.TriggerWait()
		}

		{ // Create a draft invoice for the 9th Uplink.
			user := uplinks[8].Projects[0].Owner
			inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(
				ctx, user.ID, 1000, "test invoice",
			)
			require.NoError(t, err)
			require.Equal(t, payments.InvoiceStatusDraft, inv.Status)
		}

		{ // Create an open invoice for the 10th Uplink.
			user := uplinks[9].Projects[0].Owner
			inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(
				ctx, user.ID, 1000, "test invoice",
			)
			require.NoError(t, err)

			// attempting to pay a draft invoice changes it to open if payment fails
			_, err = sat.API.Payments.StripeService.Accounts().Invoices().Pay(
				ctx, inv.ID, storjstripe.MockInvoicesPayFailure,
			)
			require.Error(t, err)

			inv, err = sat.API.Payments.StripeService.Accounts().Invoices().Get(ctx, inv.ID)
			require.NoError(t, err)
			require.Equal(t, payments.InvoiceStatusOpen, inv.Status)
		}

		{ // Create a paid invoice for the 11th Uplink.
			user := uplinks[10].Projects[0].Owner
			inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(
				ctx, user.ID, 1000, "test invoice",
			)
			require.NoError(t, err)

			_, err = sat.API.Payments.StripeService.Accounts().Invoices().Pay(
				ctx, inv.ID, storjstripe.MockInvoicesPaySuccess,
			)
			require.NoError(t, err)

			inv, err = sat.API.Payments.StripeService.Accounts().Invoices().Get(ctx, inv.ID)
			require.NoError(t, err)
			require.Equal(t, payments.InvoiceStatusPaid, inv.Status)
		}

		{ // Create a void invoice for the 12th Uplink.
			user := uplinks[11].Projects[0].Owner
			inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(
				ctx, user.ID, 1000, "test invoice",
			)
			require.NoError(t, err)

			_, err = sat.Config.Payments.MockProvider.Invoices().VoidInvoice(inv.ID, nil)
			require.NoError(t, err)

			inv, err = sat.API.Payments.StripeService.Accounts().Invoices().Get(ctx, inv.ID)
			require.NoError(t, err)
			require.Equal(t, payments.InvoiceStatusVoid, inv.Status)
		}

		{ // Create a uncollectible invoice for the 13th Uplink.
			user := uplinks[12].Projects[0].Owner
			inv, err := sat.API.Payments.StripeService.Accounts().Invoices().Create(
				ctx, user.ID, 1000, "test invoice",
			)
			require.NoError(t, err)

			// attempting to pay a draft invoice changes it to open if payment fails
			_, err = sat.Config.Payments.MockProvider.Invoices().MarkUncollectible(inv.ID, nil)
			require.NoError(t, err)

			inv, err = sat.API.Payments.StripeService.Accounts().Invoices().Get(ctx, inv.ID)
			require.NoError(t, err)
			require.Equal(t, payments.InvoiceStatusUncollectible, inv.Status)
		}

		{ // Create pending invoice items for the 14th Uplink.
			userID := uplinks[13].Projects[0].Owner.ID
			cusID, err := sat.DB.StripeCoinPayments().Customers().GetCustomerID(ctx, userID)
			require.NoError(t, err)

			amount := int64(1000)
			_, err = sat.Config.Payments.MockProvider.InvoiceItems().New(&stripe.InvoiceItemParams{
				Customer: &cusID,
				Amount:   &amount,
			})
			require.NoError(t, err)

			hasPending, err := sat.API.Payments.StripeService.Accounts().Invoices().CheckPendingItems(ctx, userID)
			require.NoError(t, err)
			require.True(t, hasPending)
		}

		// Set the accounts in "pending deletion" status, except the 15th and 16th Uplink.
		for i := 0; i < len(uplinks)-2; i++ {
			pendingStatus := console.PendingDeletion
			require.NoError(t,
				sat.DB.Console().Users().Update(ctx, uplinks[i].Projects[0].Owner.ID,
					console.UpdateUserRequest{
						Status: &pendingStatus,
					},
				),
			)
		}

		// Change some users to be in the paid tier.
		for _, i := range []int{1, 3, 7, 8, 9, 10, 11, 12, 13} {
			kind := console.PaidUser
			require.NoError(t,
				sat.DB.Console().Users().Update(ctx, uplinks[i].Projects[0].Owner.ID,
					console.UpdateUserRequest{
						Kind: &kind,
					},
				),
			)
		}

		{ // Set the 16th Uplink user in "legal hold" status.}
			pendingStatus := console.LegalHold
			require.NoError(t,
				sat.DB.Console().Users().Update(ctx, uplinks[15].Projects[0].Owner.ID,
					console.UpdateUserRequest{
						Status: &pendingStatus,
					},
				),
			)
		}

		// Create a CSV with the users' emails to delete.
		var csvData io.Reader
		{
			emails := "email"
			for _, uplnk := range uplinks {
				emails += "\n" + uplnk.User[sat.ID()].Email
			}

			csvData = bytes.NewBufferString(emails)
		}

		// Delete accounts and their associated projects.
		require.NoError(t, deleteAccounts(
			ctx, zaptest.NewLogger(t), sat.DB, sat.API.Payments.Accounts.Invoices(), csvData,
		))

		deleteVerification := func(uplinkIdx int) {
			pID := uplinks[uplinkIdx].Projects[0].ID
			keys, err := sat.DB.Console().APIKeys().GetAllNamesByProjectID(ctx, pID)
			require.NoError(t, err)
			require.Empty(t, keys)

			proj, err := sat.DB.Console().Projects().Get(ctx, pID)
			require.NoError(t, err)
			require.NotNil(t, proj.Status)
			require.Equal(t, console.ProjectDisabled, *proj.Status)

			userID := uplinks[uplinkIdx].Projects[0].Owner.ID
			user, err := sat.DB.Console().Users().Get(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, console.Deleted, user.Status)
			require.Empty(t, user.FullName)
			require.Empty(t, user.ShortName)
			require.Equal(t, user.Email, fmt.Sprintf("deactivated+%s@storj.io", user.ID))
		}

		// Verify extra project API keys are deleted.
		keys, err := sat.DB.Console().APIKeys().GetAllNamesByProjectID(ctx, extraProj.ID)
		require.NoError(t, err)
		require.Empty(t, keys)

		// Verify users and projects associated with the uplinks 1st to 6th are marked as deleted.
		for i := 0; i < 6; i++ {
			deleteVerification(i)
		}

		// Verify users and projects associated with the uplinks 11th to 13th are marked as deleted.
		for i := 10; i < 13; i++ {
			deleteVerification(i)
		}

		{ // Verify that the 6th uplink additional project is marked as deleted and their API keys
			// are deleted.
			keys, err := sat.DB.Console().APIKeys().GetAllNamesByProjectID(ctx, extraProj.ID)
			require.NoError(t, err)
			require.Empty(t, keys)

			proj, err := sat.DB.Console().Projects().Get(ctx, extraProj.ID)
			require.NoError(t, err)
			require.NotNil(t, proj.Status)
			require.Equal(t, console.ProjectDisabled, *proj.Status)
		}

		noDeleteVerification := func(uplinkIdx int, expectedStatus console.UserStatus) {
			pID := uplinks[uplinkIdx].Projects[0].ID
			keys, err := sat.DB.Console().APIKeys().GetAllNamesByProjectID(ctx, pID)
			require.NoError(t, err, uplinkIdx)
			require.NotEmpty(t, keys, uplinkIdx)

			proj, err := sat.DB.Console().Projects().Get(ctx, pID)
			require.NoError(t, err, uplinkIdx)
			require.NotNil(t, proj.Status, uplinkIdx)
			require.Equal(t, console.ProjectActive, *proj.Status, uplinkIdx)

			userID := uplinks[uplinkIdx].Projects[0].Owner.ID
			user, err := sat.DB.Console().Users().Get(ctx, userID)
			require.NoError(t, err, uplinkIdx)
			require.Equal(t, expectedStatus, user.Status, uplinkIdx)
			require.NotEmpty(t, user.FullName, uplinkIdx)
			require.NotEqual(t, user.Email, fmt.Sprintf("deactivated+%s@storj.io", userID), uplinkIdx)
		}

		// Verify that users and projects of the uplinks 7th to 10th are not marked as deleted.
		for i := 6; i < 10; i++ {
			noDeleteVerification(i, console.PendingDeletion)
		}

		{ // Verify that the 7th uplink has its data.
			buckets, err := uplinks[6].ListBuckets(ctx, sat)
			require.NoError(t, err)
			require.Len(t, buckets, 1)

			objects, err := uplinks[6].ListObjects(ctx, sat, buckets[0].Name)
			require.NoError(t, err)
			require.Len(t, objects, 1)
		}

		// Verify that users and projects of the uplinks 14th to 14th are not marked as deleted.
		for i := 13; i < 14; i++ {
			noDeleteVerification(i, console.PendingDeletion)
		}

		// Verify that users and projects of the uplinks 15th to 16th are not marked as deleted because
		// of the users statuses.
		noDeleteVerification(14, console.Active)
		noDeleteVerification(15, console.LegalHold)
	})
}

func TestSetAccountsStatusPendingDeletion(t *testing.T) {
	testCases := []struct {
		name                string
		expectedFinalStatus console.UserStatus
		initialStatus       console.UserStatus
		kind                console.UserKind
		// -1 indicates no freeze event
		expirationFreezeEvent              console.AccountFreezeEventType
		expirationFreezeDaysTillEscalation int
		memberOfThirdPartyProject          bool
	}{
		{
			name:                  "Inactive status user",
			expectedFinalStatus:   console.Inactive,
			initialStatus:         console.Inactive,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "Deleted status user",
			expectedFinalStatus:   console.Deleted,
			initialStatus:         console.Deleted,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "PendingDeletion status user",
			expectedFinalStatus:   console.PendingDeletion,
			initialStatus:         console.PendingDeletion,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "LegalHold status user",
			expectedFinalStatus:   console.LegalHold,
			initialStatus:         console.LegalHold,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "PendingBotVerification status user",
			expectedFinalStatus:   console.PendingBotVerification,
			initialStatus:         console.PendingBotVerification,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "UserRequestedDeletion status user",
			expectedFinalStatus:   console.UserRequestedDeletion,
			initialStatus:         console.UserRequestedDeletion,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "Active status user ",
			expectedFinalStatus:   console.Active,
			initialStatus:         console.Active,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "Active status user and paid tier",
			expectedFinalStatus:   console.Active,
			initialStatus:         console.Active,
			kind:                  console.PaidUser,
			expirationFreezeEvent: -1,
		},
		{
			name:                  "Active user with billing freeze",
			expectedFinalStatus:   console.Active,
			initialStatus:         console.Active,
			expirationFreezeEvent: console.BillingFreeze,
		},
		{
			name:                  "Active user with trial expiration freeze",
			expectedFinalStatus:   console.PendingDeletion,
			initialStatus:         console.Active,
			expirationFreezeEvent: console.TrialExpirationFreeze,
		},
		{
			name:                  "Active user and paid tier with trial expiration freeze",
			expectedFinalStatus:   console.Active,
			initialStatus:         console.Active,
			kind:                  console.PaidUser,
			expirationFreezeEvent: console.TrialExpirationFreeze,
		},
		{
			name:                               "Active user with trial expiration freeze with 1 day till escalation",
			expectedFinalStatus:                console.Active,
			initialStatus:                      console.Active,
			expirationFreezeEvent:              console.TrialExpirationFreeze,
			expirationFreezeDaysTillEscalation: 1,
		},
		{
			name:                      "Active user with trial expiration freeze and member of third party project ",
			expectedFinalStatus:       console.Active,
			initialStatus:             console.Active,
			expirationFreezeEvent:     console.TrialExpirationFreeze,
			memberOfThirdPartyProject: true,
		},
	}

	testplanet.Run(t, testplanet.Config{
		UplinkCount: len(testCases), SatelliteCount: 1, StorageNodeCount: 0,
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			uplinks := planet.Uplinks

			var thirdPartyProject uuid.UUID
			{
				proj, err := sat.DB.Console().Projects().Insert(ctx, &console.Project{
					Name:    "third-party-project",
					OwnerID: uplinks[0].Projects[0].Owner.ID,
				})
				require.NoError(t, err)
				thirdPartyProject = proj.ID
			}

			// Setup user statuses for all test cases
			for i, tc := range testCases {
				userID := uplinks[i].Projects[0].Owner.ID
				require.NoError(t,
					sat.DB.Console().Users().Update(ctx, userID, console.UpdateUserRequest{
						Status: &tc.initialStatus,
						Kind:   &tc.kind,
					},
					),
				)

				if tc.expirationFreezeEvent.String() != "" {
					_, err := sat.DB.Console().AccountFreezeEvents().Upsert(ctx, &console.AccountFreezeEvent{
						UserID:             userID,
						Type:               tc.expirationFreezeEvent,
						DaysTillEscalation: &tc.expirationFreezeDaysTillEscalation,
					})
					require.NoError(t, err)
				}

				if tc.memberOfThirdPartyProject {
					_, err := sat.DB.Console().ProjectMembers().Insert(
						ctx, userID, thirdPartyProject, console.RoleMember,
					)
					require.NoError(t, err)
				}
			}

			// Create a CSV with all user emails
			var csvData io.Reader
			{
				emails := "email"
				for _, uplink := range uplinks {
					emails += "\n" + uplink.User[sat.ID()].Email
				}
				csvData = bytes.NewBufferString(emails)
			}

			// Run the function under test
			require.NoError(t, setAccountsStatusPendingDeletion(
				ctx, zaptest.NewLogger(t), sat.DB, 0, csvData,
			))

			// Verify all test cases
			for i, tc := range testCases {
				userID := uplinks[i].Projects[0].Owner.ID
				user, err := sat.DB.Console().Users().Get(ctx, userID)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedFinalStatus, user.Status,
					"Test case (%d) '%s': expected status %v, got %v", i, tc.name, tc.expectedFinalStatus, user.Status,
				)
			}
		})
}

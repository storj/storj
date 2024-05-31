// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"database/sql"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetExpiresBeforeWithStatus(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		users := db.Console().Users()

		// insert paid_tier user to ensure it is never returned from GetExpiresBeforeWithStatus
		proUser := testrand.UUID()
		_, err := users.Insert(ctx, &console.User{
			ID:           proUser,
			FullName:     "test",
			Email:        "userone@mail.test",
			PasswordHash: []byte("testpassword"),
		})
		require.NoError(t, err)

		boolPtr := true
		require.NoError(t, users.Update(ctx, proUser, console.UpdateUserRequest{
			PaidTier: &boolPtr,
		}))

		u, err := users.Get(ctx, proUser)
		require.NoError(t, err)
		require.True(t, u.PaidTier)
		require.Nil(t, u.TrialExpiration)
		require.Zero(t, u.TrialNotifications)

		now := time.Now()
		tomorrow := now.Add(24 * time.Hour)
		dayAfterTomorrow := tomorrow.Add(24 * time.Hour)

		// insert free trial user with no trial notification and expires tomorrow
		trialUserNeedsReminder := testrand.UUID()
		_, err = users.Insert(ctx, &console.User{
			ID:              trialUserNeedsReminder,
			FullName:        "test",
			Email:           "usertwo@mail.test",
			PasswordHash:    []byte("testpassword"),
			TrialExpiration: &tomorrow,
		})
		require.NoError(t, err)

		u, err = users.Get(ctx, trialUserNeedsReminder)
		require.NoError(t, err)
		require.False(t, u.PaidTier)
		require.Equal(t, tomorrow.Truncate(time.Millisecond), u.TrialExpiration.Truncate(time.Millisecond))
		require.Zero(t, u.TrialNotifications)

		// insert free trial user who already got reminder and expires tomorrow
		trialUserAlreadyReminded := testrand.UUID()
		_, err = users.Insert(ctx, &console.User{
			ID:              trialUserAlreadyReminded,
			FullName:        "test",
			Email:           "usertwo@mail.test",
			PasswordHash:    []byte("testpassword"),
			TrialExpiration: &tomorrow,
		})
		require.NoError(t, err)

		notifiedStatus := console.TrialExpirationReminder
		require.NoError(t, users.Update(ctx, trialUserAlreadyReminded, console.UpdateUserRequest{
			TrialNotifications: &notifiedStatus,
		}))

		u, err = users.Get(ctx, trialUserAlreadyReminded)
		require.NoError(t, err)
		require.False(t, u.PaidTier)
		require.Equal(t, tomorrow.Truncate(time.Millisecond), u.TrialExpiration.Truncate(time.Millisecond))
		require.Equal(t, int(notifiedStatus), u.TrialNotifications)

		u, err = users.Get(ctx, trialUserAlreadyReminded)
		require.NoError(t, err)
		require.Equal(t, int(console.TrialExpirationReminder), u.TrialNotifications)

		// test with var now as expiresBefore arg. Expect trialUserNeedsReminder not returned
		// since expiration, tomorrow, is after expiresBefore arg.
		needExpirationReminder, err := users.GetExpiresBeforeWithStatus(ctx, console.NoTrialNotification, now)
		require.NoError(t, err)
		require.Len(t, needExpirationReminder, 0)

		// test with var dayAfterTomorrow as expiresBefore arg. Expect trialUserNeedsReminder returned
		// since expiration, tomorrow, is before expiresBefore arg and trial_notifications matches notificationStatus arg.
		needExpirationReminder, err = users.GetExpiresBeforeWithStatus(ctx, console.NoTrialNotification, dayAfterTomorrow)
		require.NoError(t, err)
		require.Len(t, needExpirationReminder, 1)
		require.Equal(t, trialUserNeedsReminder, needExpirationReminder[0].ID)

		// test with var now as expiresBefore arg. Expect trialUserAlreadyReminded not returned
		// since expiration, tomorrow, is after expiresBefore arg.
		needExpiredNotification, err := users.GetExpiresBeforeWithStatus(ctx, console.TrialExpirationReminder, now)
		require.NoError(t, err)
		require.Len(t, needExpiredNotification, 0)

		needExpiredNotification, err = users.GetExpiresBeforeWithStatus(ctx, console.TrialExpirationReminder, dayAfterTomorrow)
		require.NoError(t, err)
		require.Len(t, needExpiredNotification, 1)
		require.Equal(t, trialUserAlreadyReminded, needExpiredNotification[0].ID)
	})
}

func TestGetUnverifiedNeedingReminderCutoff(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		users := db.Console().Users()

		id := testrand.UUID()
		_, err := users.Insert(ctx, &console.User{
			ID:           id,
			FullName:     "test",
			Email:        "userone@mail.test",
			PasswordHash: []byte("testpassword"),
		})
		require.NoError(t, err)

		u, err := users.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, console.UserStatus(0), u.Status)

		now := time.Now()
		reminders := now.Add(time.Hour)

		// to get a reminder, created_at needs be after cutoff.
		// since we don't have control over created_at, make cutoff in the future to test that
		// user doesn't get a reminder.
		cutoff := now.Add(time.Hour)

		needingReminder, err := users.GetUnverifiedNeedingReminder(ctx, reminders, reminders, cutoff)
		require.NoError(t, err)
		require.Len(t, needingReminder, 0)

		// change cutoff so user created_at is after it.
		// user should get a reminder.
		cutoff = now.Add(-time.Hour)

		needingReminder, err = users.GetUnverifiedNeedingReminder(ctx, now, now, cutoff)
		require.NoError(t, err)
		require.Len(t, needingReminder, 1)
	})
}

func TestUpdateUser(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		users := db.Console().Users()
		id := testrand.UUID()
		u, err := users.Insert(ctx, &console.User{
			ID:               id,
			FullName:         "testFullName",
			ShortName:        "testShortName",
			Email:            "test@storj.test",
			PasswordHash:     []byte("testPasswordHash"),
			DefaultPlacement: 12,
		})
		require.NoError(t, err)

		now := time.Now()
		newInfo := console.User{
			FullName:               "updatedFullName",
			ShortName:              "updatedShortName",
			PasswordHash:           []byte("updatedPasswordHash"),
			ProjectLimit:           1,
			ProjectBandwidthLimit:  1,
			ProjectStorageLimit:    1,
			ProjectSegmentLimit:    1,
			PaidTier:               true,
			MFAEnabled:             true,
			MFASecretKey:           "secretKey",
			MFARecoveryCodes:       []string{"code1", "code2"},
			FailedLoginCount:       1,
			LoginLockoutExpiration: now.Truncate(time.Second),
			DefaultPlacement:       13,

			HaveSalesContact: true,
			IsProfessional:   true,
			Position:         "Engineer",
			CompanyName:      "Storj",
			EmployeeCount:    "1-200",

			TrialNotifications: 1,
			TrialExpiration:    &now,
			UpgradeTime:        &now,
		}

		require.NotEqual(t, u.FullName, newInfo.FullName)
		require.NotEqual(t, u.ShortName, newInfo.ShortName)
		require.NotEqual(t, u.PasswordHash, newInfo.PasswordHash)
		require.NotEqual(t, u.ProjectLimit, newInfo.ProjectLimit)
		require.NotEqual(t, u.ProjectBandwidthLimit, newInfo.ProjectBandwidthLimit)
		require.NotEqual(t, u.ProjectStorageLimit, newInfo.ProjectStorageLimit)
		require.NotEqual(t, u.ProjectSegmentLimit, newInfo.ProjectSegmentLimit)
		require.NotEqual(t, u.PaidTier, newInfo.PaidTier)
		require.NotEqual(t, u.MFAEnabled, newInfo.MFAEnabled)
		require.NotEqual(t, u.MFASecretKey, newInfo.MFASecretKey)
		require.NotEqual(t, u.MFARecoveryCodes, newInfo.MFARecoveryCodes)
		require.NotEqual(t, u.FailedLoginCount, newInfo.FailedLoginCount)
		require.NotEqual(t, u.LoginLockoutExpiration, newInfo.LoginLockoutExpiration)
		require.NotEqual(t, u.DefaultPlacement, newInfo.DefaultPlacement)
		require.NotEqual(t, u.IsProfessional, newInfo.IsProfessional)
		require.NotEqual(t, u.Position, newInfo.Position)
		require.NotEqual(t, u.CompanyName, newInfo.CompanyName)
		require.NotEqual(t, u.EmployeeCount, newInfo.EmployeeCount)
		require.NotEqual(t, u.TrialNotifications, newInfo.TrialNotifications)
		require.NotEqual(t, u.TrialExpiration, newInfo.TrialExpiration)
		require.NotEqual(t, u.UpgradeTime, newInfo.UpgradeTime)

		// update just fullname
		updateReq := console.UpdateUserRequest{
			FullName: &newInfo.FullName,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err := users.Get(ctx, id)
		require.NoError(t, err)

		u.FullName = newInfo.FullName
		require.Equal(t, u, updatedUser)

		// update just shortname
		shortNamePtr := &newInfo.ShortName
		updateReq = console.UpdateUserRequest{
			ShortName: &shortNamePtr,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ShortName = newInfo.ShortName
		require.Equal(t, u, updatedUser)

		// update just password hash
		updateReq = console.UpdateUserRequest{
			PasswordHash: newInfo.PasswordHash,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.PasswordHash = newInfo.PasswordHash
		require.Equal(t, u, updatedUser)

		// update just project limit
		updateReq = console.UpdateUserRequest{
			ProjectLimit: &newInfo.ProjectLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectLimit = newInfo.ProjectLimit
		require.Equal(t, u, updatedUser)

		// update just project bw limit
		updateReq = console.UpdateUserRequest{
			ProjectBandwidthLimit: &newInfo.ProjectBandwidthLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectBandwidthLimit = newInfo.ProjectBandwidthLimit
		require.Equal(t, u, updatedUser)

		// update just project storage limit
		updateReq = console.UpdateUserRequest{
			ProjectStorageLimit: &newInfo.ProjectStorageLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectStorageLimit = newInfo.ProjectStorageLimit
		require.Equal(t, u, updatedUser)

		// update just project segment limit
		updateReq = console.UpdateUserRequest{
			ProjectSegmentLimit: &newInfo.ProjectSegmentLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectSegmentLimit = newInfo.ProjectSegmentLimit
		require.Equal(t, u, updatedUser)

		// update just paid tier
		updateReq = console.UpdateUserRequest{
			PaidTier: &newInfo.PaidTier,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.PaidTier = newInfo.PaidTier
		require.Equal(t, u, updatedUser)

		// update just mfa enabled
		updateReq = console.UpdateUserRequest{
			MFAEnabled: &newInfo.MFAEnabled,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.MFAEnabled = newInfo.MFAEnabled
		require.Equal(t, u, updatedUser)

		// update just mfa secret key
		secretKeyPtr := &newInfo.MFASecretKey
		updateReq = console.UpdateUserRequest{
			MFASecretKey: &secretKeyPtr,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.MFASecretKey = newInfo.MFASecretKey
		require.Equal(t, u, updatedUser)

		// update just mfa recovery codes
		updateReq = console.UpdateUserRequest{
			MFARecoveryCodes: &newInfo.MFARecoveryCodes,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.MFARecoveryCodes = newInfo.MFARecoveryCodes
		require.Equal(t, u, updatedUser)

		// update just failed login count
		updateReq = console.UpdateUserRequest{
			FailedLoginCount: &newInfo.FailedLoginCount,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.FailedLoginCount = newInfo.FailedLoginCount
		require.Equal(t, u, updatedUser)

		// update just login lockout expiration
		loginLockoutExpPtr := &newInfo.LoginLockoutExpiration
		updateReq = console.UpdateUserRequest{
			LoginLockoutExpiration: &loginLockoutExpPtr,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.LoginLockoutExpiration = newInfo.LoginLockoutExpiration
		require.Equal(t, u, updatedUser)

		// update just the placement
		defaultPlacement := &newInfo.DefaultPlacement
		updateReq = console.UpdateUserRequest{
			DefaultPlacement: *defaultPlacement,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.DefaultPlacement = newInfo.DefaultPlacement
		require.Equal(t, u, updatedUser)

		// update professional info
		updateReq = console.UpdateUserRequest{
			IsProfessional:   &newInfo.IsProfessional,
			HaveSalesContact: &newInfo.HaveSalesContact,
			Position:         &newInfo.Position,
			CompanyName:      &newInfo.CompanyName,
			EmployeeCount:    &newInfo.EmployeeCount,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.HaveSalesContact = newInfo.HaveSalesContact
		u.IsProfessional = newInfo.IsProfessional
		u.Position = newInfo.Position
		u.CompanyName = newInfo.CompanyName
		u.EmployeeCount = newInfo.EmployeeCount
		require.Equal(t, u, updatedUser)

		// update trial expiration and upgrade time.
		newDate := now.Add(time.Hour)
		newDatePtr := &newDate
		updateReq = console.UpdateUserRequest{
			TrialExpiration: &newDatePtr,
			UpgradeTime:     &newDate,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)
		require.WithinDuration(t, newDate, *updatedUser.TrialExpiration, time.Minute)
		require.WithinDuration(t, newDate, *updatedUser.UpgradeTime, time.Minute)
	})
}

func TestUpdateUserProjectLimits(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		limits := console.UsageLimits{Storage: rand.Int63(), Bandwidth: rand.Int63(), Segment: rand.Int63()}
		usersRepo := db.Console().Users()

		user, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "User",
			Email:        "test@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		err = usersRepo.UpdateUserProjectLimits(ctx, user.ID, limits)
		require.NoError(t, err)

		user, err = usersRepo.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, limits.Bandwidth, user.ProjectBandwidthLimit)
		require.Equal(t, limits.Storage, user.ProjectStorageLimit)
		require.Equal(t, limits.Segment, user.ProjectSegmentLimit)
	})
}

func TestUpdateDefaultPlacement(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()

		user, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "User",
			Email:        "test@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		err = usersRepo.UpdateDefaultPlacement(ctx, user.ID, 12)
		require.NoError(t, err)

		user, err = usersRepo.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, storj.PlacementConstraint(12), user.DefaultPlacement)

		err = usersRepo.UpdateDefaultPlacement(ctx, user.ID, storj.EveryCountry)
		require.NoError(t, err)

		user, err = usersRepo.Get(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, storj.EveryCountry, user.DefaultPlacement)
	})
}

func TestGetUpgradeTime(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()

		user, err := usersRepo.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "User",
			Email:        "test@mail.test",
			PasswordHash: []byte("123a123"),
		})
		require.NoError(t, err)

		upgradeTime, err := usersRepo.GetUpgradeTime(ctx, user.ID)
		require.NoError(t, err)
		require.Nil(t, upgradeTime)

		now := time.Now()

		err = usersRepo.Update(ctx, user.ID, console.UpdateUserRequest{UpgradeTime: &now})
		require.NoError(t, err)

		upgradeTime, err = usersRepo.GetUpgradeTime(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, upgradeTime)
		require.WithinDuration(t, now, *upgradeTime, time.Minute)
	})
}

func TestUserSettings(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		users := db.Console().Users()
		id := testrand.UUID()
		sessionDur := time.Duration(rand.Int63()).Round(time.Minute)
		sessionDurPtr := &sessionDur
		var nilDur *time.Duration

		_, err := users.GetSettings(ctx, id)
		require.ErrorIs(t, err, sql.ErrNoRows)

		for _, tt := range []struct {
			name     string
			upserted **time.Duration
			expected *time.Duration
		}{
			{"update when given pointer to non-nil value", &sessionDurPtr, sessionDurPtr},
			{"ignore when given nil pointer", nil, sessionDurPtr},
			{"nullify when given pointer to nil", &nilDur, nil},
		} {
			t.Run(tt.name, func(t *testing.T) {
				require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{
					SessionDuration: tt.upserted,
				}))
				settings, err := users.GetSettings(ctx, id)
				require.NoError(t, err)
				require.Equal(t, tt.expected, settings.SessionDuration)
			})
		}

		t.Run("test onboarding", func(t *testing.T) {
			id = testrand.UUID()
			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{}))
			settings, err := users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.False(t, settings.OnboardingStart)
			require.False(t, settings.OnboardingEnd)
			require.Nil(t, settings.OnboardingStep)

			newBool := true
			newStep := "Overview"
			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{
				OnboardingStart: &newBool,
				OnboardingEnd:   &newBool,
				OnboardingStep:  &newStep,
			}))
			settings, err = users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.Equal(t, newBool, settings.OnboardingStart)
			require.Equal(t, newBool, settings.OnboardingEnd)
			require.Equal(t, &newStep, settings.OnboardingStep)
		})

		t.Run("test passphrase prompt", func(t *testing.T) {
			id = testrand.UUID()
			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{}))
			settings, err := users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.True(t, settings.PassphrasePrompt)

			newBool := false
			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{
				PassphrasePrompt: &newBool,
			}))
			settings, err = users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.Equal(t, newBool, settings.PassphrasePrompt)

			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{}))
			settings, err = users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.Equal(t, newBool, settings.PassphrasePrompt)
		})

		t.Run("test notice dismissal", func(t *testing.T) {
			id = testrand.UUID()
			noticeDismissal := console.NoticeDismissal{
				FileGuide:                false,
				ServerSideEncryption:     false,
				PartnerUpgradeBanner:     false,
				ProjectMembersPassphrase: false,
				UploadOverwriteWarning:   false,
				VersioningBetaBanner:     false,
			}

			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{}))
			settings, err := users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.Equal(t, noticeDismissal, settings.NoticeDismissal)

			noticeDismissal.FileGuide = true
			noticeDismissal.ServerSideEncryption = true
			noticeDismissal.PartnerUpgradeBanner = true
			noticeDismissal.ProjectMembersPassphrase = true
			noticeDismissal.UploadOverwriteWarning = true
			noticeDismissal.VersioningBetaBanner = true
			require.NoError(t, users.UpsertSettings(ctx, id, console.UpsertUserSettingsRequest{
				NoticeDismissal: &noticeDismissal,
			}))
			settings, err = users.GetSettings(ctx, id)
			require.NoError(t, err)
			require.Equal(t, noticeDismissal, settings.NoticeDismissal)
		})
	})
}

func TestDeleteUnverifiedBefore(t *testing.T) {
	maxUnverifiedAge := time.Hour
	now := time.Now()
	expiration := now.Add(-maxUnverifiedAge)

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersDB := db.Console().Users()
		now := time.Now()

		// Only positive page sizes should be allowed.
		require.Error(t, usersDB.DeleteUnverifiedBefore(ctx, time.Time{}, 0, 0))
		require.Error(t, usersDB.DeleteUnverifiedBefore(ctx, time.Time{}, 0, -1))

		createUser := func(status console.UserStatus, createdAt time.Time) uuid.UUID {
			user, err := usersDB.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				PasswordHash: testrand.Bytes(8),
			})
			require.NoError(t, err)

			result, err := db.Testing().RawDB().ExecContext(ctx,
				"UPDATE users SET created_at = $1, status = $2 WHERE id = $3",
				createdAt, status, user.ID,
			)
			require.NoError(t, err)

			count, err := result.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 1, count)

			return user.ID
		}

		oldActive := createUser(console.Active, expiration.Add(-time.Second))
		newUnverified := createUser(console.Inactive, now)
		oldUnverified := createUser(console.Inactive, expiration.Add(-time.Second))

		require.NoError(t, usersDB.DeleteUnverifiedBefore(ctx, expiration, 0, 1))

		// Ensure that the old, unverified user record was deleted and the others remain.
		_, err := usersDB.Get(ctx, oldUnverified)
		require.ErrorIs(t, err, sql.ErrNoRows)
		_, err = usersDB.Get(ctx, newUnverified)
		require.NoError(t, err)
		_, err = usersDB.Get(ctx, oldActive)
		require.NoError(t, err)
	})
}

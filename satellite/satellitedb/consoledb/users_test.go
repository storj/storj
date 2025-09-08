// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

		kind := console.PaidUser
		require.NoError(t, users.Update(ctx, proUser, console.UpdateUserRequest{
			Kind: &kind,
		}))

		u, err := users.Get(ctx, proUser)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, u.Kind)
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
		require.Equal(t, console.FreeUser, u.Kind)
		require.WithinDuration(t, tomorrow.Truncate(time.Millisecond), u.TrialExpiration.Truncate(time.Millisecond), time.Nanosecond)
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
		require.Equal(t, console.FreeUser, u.Kind)
		require.WithinDuration(t, tomorrow.Truncate(time.Millisecond), u.TrialExpiration.Truncate(time.Millisecond), time.Nanosecond)
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

// Spanner does not record time zones and instead uses an absolute time system. TIMESTAMP values are always returned via time.Time
// values in UTC (see: https://pkg.go.dev/cloud.google.com/go/spanner#Row). pgx, for Postgres/Cockroach/etc., returns time.Time
// values always in the session local time, time.Local, (see: https://github.com/jackc/pgx/issues/2117). As such, we can't use require.Equal
// for comparing two time.Time values as require.Equal uses strict equality (two objects are only equal if ALL fields recursively are identical).
// e.g. two time.Time objects representing the same instant of time in two different timezones will evaluate to false by require.Equal.
// Locally instantiated time.Time objects have their timezone as time.Local so in order to test the true intent, that the two time.Time
// objects represent the same instant of time (regardless of timezone), we use cmp.Diff with cmpopts.EquateApproxTime below as well as other places.
func usersAreEqual(t *testing.T, expected, actual *console.User) {
	require.Equal(t, "", cmp.Diff(actual, expected, cmpopts.EquateApproxTime(0)))
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
			Kind:                   console.PaidUser,
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
		require.NotEqual(t, u.Kind, newInfo.Kind)
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
		usersAreEqual(t, u, updatedUser)

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
		usersAreEqual(t, u, updatedUser)

		// update just password hash
		updateReq = console.UpdateUserRequest{
			PasswordHash: newInfo.PasswordHash,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.PasswordHash = newInfo.PasswordHash
		usersAreEqual(t, u, updatedUser)

		// update just project limit
		updateReq = console.UpdateUserRequest{
			ProjectLimit: &newInfo.ProjectLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectLimit = newInfo.ProjectLimit
		usersAreEqual(t, u, updatedUser)

		// update just project bw limit
		updateReq = console.UpdateUserRequest{
			ProjectBandwidthLimit: &newInfo.ProjectBandwidthLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectBandwidthLimit = newInfo.ProjectBandwidthLimit
		usersAreEqual(t, u, updatedUser)

		// update just project storage limit
		updateReq = console.UpdateUserRequest{
			ProjectStorageLimit: &newInfo.ProjectStorageLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectStorageLimit = newInfo.ProjectStorageLimit
		usersAreEqual(t, u, updatedUser)

		// update just project segment limit
		updateReq = console.UpdateUserRequest{
			ProjectSegmentLimit: &newInfo.ProjectSegmentLimit,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.ProjectSegmentLimit = newInfo.ProjectSegmentLimit
		usersAreEqual(t, u, updatedUser)

		// update just paid tier
		updateReq = console.UpdateUserRequest{
			Kind: &newInfo.Kind,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.Kind = newInfo.Kind
		usersAreEqual(t, u, updatedUser)

		// update just mfa enabled
		updateReq = console.UpdateUserRequest{
			MFAEnabled: &newInfo.MFAEnabled,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.MFAEnabled = newInfo.MFAEnabled
		usersAreEqual(t, u, updatedUser)

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
		usersAreEqual(t, u, updatedUser)

		// update just mfa recovery codes
		updateReq = console.UpdateUserRequest{
			MFARecoveryCodes: &newInfo.MFARecoveryCodes,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.MFARecoveryCodes = newInfo.MFARecoveryCodes
		usersAreEqual(t, u, updatedUser)

		// update just failed login count
		updateReq = console.UpdateUserRequest{
			FailedLoginCount: &newInfo.FailedLoginCount,
		}

		err = users.Update(ctx, id, updateReq)
		require.NoError(t, err)

		updatedUser, err = users.Get(ctx, id)
		require.NoError(t, err)

		u.FailedLoginCount = newInfo.FailedLoginCount
		usersAreEqual(t, u, updatedUser)

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
		usersAreEqual(t, u, updatedUser)

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
		usersAreEqual(t, u, updatedUser)

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
		usersAreEqual(t, u, updatedUser)

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
				FileGuide:                        false,
				ServerSideEncryption:             false,
				PartnerUpgradeBanner:             false,
				ProjectMembersPassphrase:         false,
				UploadOverwriteWarning:           false,
				ObjectMountConsultationRequested: false,
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
			noticeDismissal.ObjectMountConsultationRequested = true
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
				db.Testing().Rebind("UPDATE users SET created_at = ?, status = ? WHERE id = ?"),
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

func TestUsersSetStatusPendingDeletion(t *testing.T) {
	t.Parallel()
	// There are 2 loops around satellitedbtest.Run and 3 inside because having all of them inside
	// exhaust the Spanner emulator. I don't know the reasons, I founded with a trial and error
	// approach.
	for _, kind := range []console.UserKind{console.FreeUser, console.PaidUser} {
		for status := console.UserStatus(0); status < console.UserStatusCount; status++ {
			t.Run(fmt.Sprintf("kind=%v,status=%v", kind, status), func(t *testing.T) {
				testUsersSetStatusPendingDeletion(t, kind, status)
			})
		}
	}
}

func testUsersSetStatusPendingDeletion(t *testing.T, kind console.UserKind, status console.UserStatus) {
	const defaultDaysTillEscalation = 2

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()

		var thirdPartyProject uuid.UUID
		{ // Create a user and project to add the users of the following combination of tests to be
			// member of it for the project member test combinations.
			user, err := usersRepo.Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				FullName:     "Third party project owner",
				Email:        "third-party-project-owner@mail.test",
				PasswordHash: []byte("password"),
			})
			require.NoError(t, err)

			project, err := db.Console().Projects().Insert(
				ctx, &console.Project{
					Name:    "third-party-project",
					OwnerID: user.ID,
				},
			)
			require.NoError(t, err)

			thirdPartyProject = project.ID
		}

		userStatus := status

		// -1 means not being a member.
		for m := -1; m <= int(console.RoleMember); m++ {
			memberType := console.ProjectMemberRole(m)

			// This loop finishes by a conditional at the end of the loop block.
			for event := console.AccountFreezeEventType(0); true; event++ {
				// Days until escalation. -1 is null in the DB
				for days := -1; days <= 1; days++ {
					t.Run(fmt.Sprintf(
						"PaidTier=%t_Status=%s_Member=%s_Event=%s=DaysUntilEscalation=%d",
						kind == console.PaidUser, userStatus.String(), memberType, event, days,
					), func(t *testing.T) {
						// Create user and set the account freeze event.
						user, err := usersRepo.Insert(ctx, &console.User{
							ID:           testrand.UUID(),
							FullName:     "Test User",
							Email:        fmt.Sprintf("test-%s@mail.test", testrand.UUID().String()),
							PasswordHash: []byte("password"),
							Kind:         kind,
						})
						require.NoError(t, err)

						{ // Set the status because Insert ignores the status field.
							updateReq := console.UpdateUserRequest{
								Status: &userStatus,
							}
							err = usersRepo.Update(ctx, user.ID, updateReq)
							require.NoError(t, err)
						}

						{ // Create a project and create an entry in project members because all the owners are
							// admin member of their projects
							project, err := db.Console().Projects().Insert(
								ctx, &console.Project{
									Name:    user.Email,
									OwnerID: user.ID,
								},
							)
							require.NoError(t, err)

							_, err = db.Console().ProjectMembers().Insert(
								ctx, user.ID, project.ID, console.RoleAdmin,
							)
							require.NoError(t, err)
						}

						if memberType.String() != "" {
							_, err = db.Console().ProjectMembers().Insert(ctx, user.ID, thirdPartyProject, memberType)
							require.NoError(t, err)
						}

						// We check that's a valid event, otherwise we skip it to have checks without without a
						// freeze event.
						if event.String() != "" {
							var daysp *int
							if days >= 0 {
								daysp = &days
							}
							_, err = db.Console().AccountFreezeEvents().Upsert(ctx, &console.AccountFreezeEvent{
								UserID: user.ID,
								Type:   event,
								Limits: &console.AccountFreezeEventLimits{
									User:     console.UsageLimits{},
									Projects: make(map[uuid.UUID]console.UsageLimits),
								},
								DaysTillEscalation: daysp,
							})
							require.NoError(t, err)
						}

						// Call SetSatusPendingDeletion for this user.
						err = db.Console().Users().SetStatusPendingDeletion(ctx, user.ID, defaultDaysTillEscalation)

						// Verify the result.
						require.GreaterOrEqual(t, defaultDaysTillEscalation, 2,
							"days till escalation is required to be at least 2 for this test setup",
						)
						if kind == console.FreeUser &&
							userStatus == console.Active &&
							event == console.TrialExpirationFreeze &&
							days == 0 &&
							memberType.String() == "" {

							require.NoError(t, err)

							updatedUser, err := db.Console().Users().Get(ctx, user.ID)
							require.NoError(t, err)
							require.Equal(t, console.PendingDeletion, updatedUser.Status)
							require.NotNil(t, updatedUser.StatusUpdatedAt, "StatusUpdateAt")
							// Delta is 10 seconds to reduce the chances to fail test because of the Spanner emulator
							// running slower.
							require.WithinDuration(
								t, time.Now().UTC(), *updatedUser.StatusUpdatedAt, 10*time.Second, "StatusUpdatedAt",
							)
						} else {
							require.ErrorIs(t, err, sql.ErrNoRows)
						}
					})
				}

				// If the event doesn't have a string representation is an invalid event. Event values
				// are consecutive positive integers, hence, the previous iteration reached the event
				// type with maximum value and this one was to check a user without a freeze event.
				if event.String() == "" {
					break
				}
			}
		}
	})
}

func TestUsersSetStatusPendingDeletion_UserMissing(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		t.Run("unexisting_user_account", func(t *testing.T) {
			err := db.Console().Users().SetStatusPendingDeletion(ctx, testrand.UUID(), 0)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})
	})
}

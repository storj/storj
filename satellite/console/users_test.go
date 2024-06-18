// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const (
	lastName        = "lastName"
	email           = "email@mail.test"
	passValid       = "password"
	name            = "name"
	newName         = "newName"
	newLastName     = "newLastName"
	newEmail        = "newEmail@mail.test"
	newPass         = "newPass1234567890123456789012345"
	position        = "position"
	companyName     = "companyName"
	employeeCount   = "0"
	workingOn       = "workingOn"
	isProfessional  = true
	mfaSecretKey    = "mfaSecretKey"
	signupPromoCode = "STORJ50"
)

func TestUserRepository(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		repository := db.Console().Users()

		user := &console.User{
			ID:           testrand.UUID(),
			FullName:     name,
			ShortName:    lastName,
			Email:        email,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}
		testUsers(ctx, t, repository, user)

		// test professional user
		user = &console.User{
			ID:              testrand.UUID(),
			FullName:        name,
			ShortName:       lastName,
			Email:           email,
			PasswordHash:    []byte(passValid),
			CreatedAt:       time.Now(),
			IsProfessional:  isProfessional,
			Position:        position,
			CompanyName:     companyName,
			EmployeeCount:   employeeCount,
			WorkingOn:       workingOn,
			SignupPromoCode: signupPromoCode,
		}
		testUsers(ctx, t, repository, user)
	})
}

func TestUserEmailCase(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		for _, testCase := range []struct {
			email string
		}{
			{email: "prettyandsimple@example.test"},
			{email: "firstname.lastname@domain.test	"},
			{email: "email@subdomain.domain.test	"},
			{email: "firstname+lastname@domain.test	"},
			{email: "email@[123.123.123.123]	"},
			{email: "\"email\"@domain.test"},
			{email: "_______@domain.test	"},
		} {
			newUser := &console.User{
				ID:           testrand.UUID(),
				FullName:     newName,
				ShortName:    newLastName,
				Email:        testCase.email,
				Status:       console.Active,
				PasswordHash: []byte(newPass),
			}

			createdUser, err := db.Console().Users().Insert(ctx, newUser)
			assert.NoError(t, err)
			assert.Equal(t, testCase.email, createdUser.Email)

			createdUser.Status = console.Active

			err = db.Console().Users().Update(ctx, createdUser.ID, console.UpdateUserRequest{
				Status: &createdUser.Status,
			})
			assert.NoError(t, err)

			retrievedUser, err := db.Console().Users().GetByEmail(ctx, testCase.email)
			assert.NoError(t, err)
			assert.Equal(t, testCase.email, retrievedUser.Email)
		}
	})
}

func TestUserUpdatePaidTier(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		email := "testemail@mail.test"
		fullName := "first name last name"
		shortName := "short name"
		password := "password"
		projectBandwidthLimit := memory.Size(50000000000)
		storageStorageLimit := memory.Size(50000000000)
		projectLimit := 3
		segmentLimit := int64(100)
		newUser := &console.User{
			ID:           testrand.UUID(),
			FullName:     fullName,
			ShortName:    shortName,
			Email:        email,
			Status:       console.Active,
			PasswordHash: []byte(password),
		}

		createdUser, err := db.Console().Users().Insert(ctx, newUser)
		require.NoError(t, err)
		require.Equal(t, email, createdUser.Email)
		require.Equal(t, fullName, createdUser.FullName)
		require.Equal(t, shortName, createdUser.ShortName)
		require.False(t, createdUser.PaidTier)

		now := time.Now()
		err = db.Console().Users().UpdatePaidTier(ctx, createdUser.ID, true, projectBandwidthLimit, storageStorageLimit, segmentLimit, projectLimit, &now)
		require.NoError(t, err)

		retrievedUser, err := db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.Equal(t, email, retrievedUser.Email)
		require.Equal(t, fullName, retrievedUser.FullName)
		require.Equal(t, shortName, retrievedUser.ShortName)
		require.True(t, retrievedUser.PaidTier)
		require.WithinDuration(t, now, *retrievedUser.UpgradeTime, time.Minute)

		err = db.Console().Users().UpdatePaidTier(ctx, createdUser.ID, false, projectBandwidthLimit, storageStorageLimit, segmentLimit, projectLimit, nil)
		require.NoError(t, err)

		retrievedUser, err = db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.False(t, retrievedUser.PaidTier)
		require.WithinDuration(t, now, *retrievedUser.UpgradeTime, time.Minute)
	})
}

func testUsers(ctx context.Context, t *testing.T, repository console.Users, user *console.User) {

	t.Run("User insertion success", func(t *testing.T) {

		insertedUser, err := repository.Insert(ctx, user)
		assert.NoError(t, err)

		insertedUser.Status = console.Active

		err = repository.Update(ctx, insertedUser.ID, console.UpdateUserRequest{
			Status: &insertedUser.Status,
		})
		assert.NoError(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByEmail, err := repository.GetByEmail(ctx, email)
		assert.NoError(t, err)
		assert.Equal(t, name, userByEmail.FullName)
		assert.Equal(t, lastName, userByEmail.ShortName)
		assert.Equal(t, user.SignupPromoCode, userByEmail.SignupPromoCode)
		assert.False(t, user.PaidTier)
		assert.False(t, user.MFAEnabled)
		assert.Empty(t, user.MFASecretKey)
		assert.Empty(t, user.MFARecoveryCodes)

		if user.IsProfessional {
			assert.Equal(t, workingOn, userByEmail.WorkingOn)
			assert.Equal(t, position, userByEmail.Position)
			assert.Equal(t, companyName, userByEmail.CompanyName)
			assert.Equal(t, employeeCount, userByEmail.EmployeeCount)
		} else {
			assert.Equal(t, "", userByEmail.WorkingOn)
			assert.Equal(t, "", userByEmail.Position)
			assert.Equal(t, "", userByEmail.CompanyName)
			assert.Equal(t, "", userByEmail.EmployeeCount)
		}

		userByID, err := repository.Get(ctx, userByEmail.ID)
		assert.NoError(t, err)
		assert.Equal(t, name, userByID.FullName)
		assert.Equal(t, lastName, userByID.ShortName)
		assert.Equal(t, user.SignupPromoCode, userByID.SignupPromoCode)
		assert.False(t, user.MFAEnabled)
		assert.Empty(t, user.MFASecretKey)
		assert.Empty(t, user.MFARecoveryCodes)

		if user.IsProfessional {
			assert.Equal(t, workingOn, userByID.WorkingOn)
			assert.Equal(t, position, userByID.Position)
			assert.Equal(t, companyName, userByID.CompanyName)
			assert.Equal(t, employeeCount, userByID.EmployeeCount)
		} else {
			assert.Equal(t, "", userByID.WorkingOn)
			assert.Equal(t, "", userByID.Position)
			assert.Equal(t, "", userByID.CompanyName)
			assert.Equal(t, "", userByID.EmployeeCount)
		}

		assert.Equal(t, userByID.ID, userByEmail.ID)
		assert.Equal(t, userByID.FullName, userByEmail.FullName)
		assert.Equal(t, userByID.ShortName, userByEmail.ShortName)
		assert.Equal(t, userByID.Email, userByEmail.Email)
		assert.Equal(t, userByID.PasswordHash, userByEmail.PasswordHash)
		assert.Equal(t, userByID.CreatedAt, userByEmail.CreatedAt)
		assert.Equal(t, userByID.IsProfessional, userByEmail.IsProfessional)
		assert.Equal(t, userByID.WorkingOn, userByEmail.WorkingOn)
		assert.Equal(t, userByID.Position, userByEmail.Position)
		assert.Equal(t, userByID.CompanyName, userByEmail.CompanyName)
		assert.Equal(t, userByID.EmployeeCount, userByEmail.EmployeeCount)
		assert.Equal(t, userByID.SignupPromoCode, userByEmail.SignupPromoCode)
	})

	t.Run("Update user success", func(t *testing.T) {
		oldUser, err := repository.GetByEmail(ctx, email)
		assert.NoError(t, err)

		newUserInfo := &console.User{
			ID:               oldUser.ID,
			FullName:         newName,
			ShortName:        newLastName,
			Email:            newEmail,
			Status:           console.Active,
			PaidTier:         true,
			MFAEnabled:       true,
			MFASecretKey:     mfaSecretKey,
			MFARecoveryCodes: []string{"1", "2"},
			PasswordHash:     []byte(newPass),
		}

		shortNamePtr := &newUserInfo.ShortName
		secretKeyPtr := &newUserInfo.MFASecretKey

		err = repository.Update(ctx, newUserInfo.ID, console.UpdateUserRequest{
			FullName:         &newUserInfo.FullName,
			ShortName:        &shortNamePtr,
			Email:            &newUserInfo.Email,
			Status:           &newUserInfo.Status,
			PaidTier:         &newUserInfo.PaidTier,
			MFAEnabled:       &newUserInfo.MFAEnabled,
			MFASecretKey:     &secretKeyPtr,
			MFARecoveryCodes: &newUserInfo.MFARecoveryCodes,
			PasswordHash:     newUserInfo.PasswordHash,
		})
		assert.NoError(t, err)

		newUser, err := repository.Get(ctx, oldUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, oldUser.ID, newUser.ID)
		assert.Equal(t, newName, newUser.FullName)
		assert.Equal(t, newLastName, newUser.ShortName)
		assert.Equal(t, newEmail, newUser.Email)
		assert.Equal(t, []byte(newPass), newUser.PasswordHash)
		assert.True(t, newUser.PaidTier)
		assert.True(t, newUser.MFAEnabled)
		assert.Equal(t, mfaSecretKey, newUser.MFASecretKey)
		assert.Equal(t, newUserInfo.MFARecoveryCodes, newUser.MFARecoveryCodes)
		assert.Equal(t, oldUser.CreatedAt, newUser.CreatedAt)
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByEmail(ctx, newEmail)
		assert.NoError(t, err)

		err = repository.Delete(ctx, oldUser.ID)
		assert.NoError(t, err)

		_, err = repository.Get(ctx, oldUser.ID)
		assert.Error(t, err)
	})
}

func TestGetUserByEmail(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()
		email := "test@mail.test"

		inactiveUser := console.User{
			ID:           testrand.UUID(),
			FullName:     "Inactive User",
			Email:        email,
			PasswordHash: []byte("password"),
		}

		_, err := usersRepo.Insert(ctx, &inactiveUser)
		require.NoError(t, err)

		_, err = usersRepo.GetByEmail(ctx, email)
		require.ErrorIs(t, sql.ErrNoRows, err)

		verified, unverified, err := usersRepo.GetByEmailWithUnverified(ctx, email)
		require.NoError(t, err)
		require.Nil(t, verified)
		require.Equal(t, inactiveUser.ID, unverified[0].ID)

		activeUser := console.User{
			ID:           testrand.UUID(),
			FullName:     "Active User",
			Email:        email,
			Status:       console.Active,
			PasswordHash: []byte("password"),
		}

		_, err = usersRepo.Insert(ctx, &activeUser)
		require.NoError(t, err)

		// Required to set the active status.
		err = usersRepo.Update(ctx, activeUser.ID, console.UpdateUserRequest{
			Status: &activeUser.Status,
		})
		require.NoError(t, err)

		dbUser, err := usersRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		require.Equal(t, activeUser.ID, dbUser.ID)
	})
}

func TestGetUsersByStatus(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()

		inactiveUser := console.User{
			ID:           testrand.UUID(),
			FullName:     "Inactive User",
			Email:        email,
			PasswordHash: []byte("password"),
		}

		_, err := usersRepo.Insert(ctx, &inactiveUser)
		require.NoError(t, err)

		activeUser := console.User{
			ID:           testrand.UUID(),
			FullName:     "Active User",
			Email:        email,
			Status:       console.Active,
			PasswordHash: []byte("password"),
		}

		_, err = usersRepo.Insert(ctx, &activeUser)
		require.NoError(t, err)

		// Required to set the active status.
		err = usersRepo.Update(ctx, activeUser.ID, console.UpdateUserRequest{
			Status: &activeUser.Status,
		})
		require.NoError(t, err)

		cursor := console.UserCursor{
			Limit: 50,
			Page:  1,
		}
		usersPage, err := usersRepo.GetByStatus(ctx, console.Inactive, cursor)
		require.NoError(t, err)
		require.Lenf(t, usersPage.Users, 1, "expected 1 inactive user")
		require.Equal(t, inactiveUser.ID, usersPage.Users[0].ID)

		usersPage, err = usersRepo.GetByStatus(ctx, console.Active, cursor)
		require.NoError(t, err)
		require.Lenf(t, usersPage.Users, 1, "expected 1 active user")
		require.Equal(t, activeUser.ID, usersPage.Users[0].ID)
	})
}

func TestGetEmailsForDeletion(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()
		now := time.Now()
		nowPlusMinute := now.Add(time.Minute)

		activeUser := console.User{
			ID:           testrand.UUID(),
			FullName:     "Active User",
			Email:        email,
			Status:       console.Active,
			PasswordHash: []byte("password"),
		}

		_, err := usersRepo.Insert(ctx, &activeUser)
		require.NoError(t, err)

		emails, err := usersRepo.GetEmailsForDeletion(ctx, now)
		require.NoError(t, err)
		require.Len(t, emails, 0)

		freeTrialUser := console.User{
			ID:              testrand.UUID(),
			FullName:        "Free Trial User",
			Email:           email + "1",
			Status:          console.UserRequestedDeletion,
			PaidTier:        false,
			PasswordHash:    []byte("password"),
			StatusUpdatedAt: &now,
		}

		_, err = usersRepo.Insert(ctx, &freeTrialUser)
		require.NoError(t, err)

		// Required to set the marked for deletion status.
		err = usersRepo.Update(ctx, freeTrialUser.ID, console.UpdateUserRequest{
			Status: &freeTrialUser.Status,
		})
		require.NoError(t, err)

		emails, err = usersRepo.GetEmailsForDeletion(ctx, now)
		require.NoError(t, err)
		require.Zero(t, len(emails))

		emails, err = usersRepo.GetEmailsForDeletion(ctx, nowPlusMinute)
		require.NoError(t, err)
		require.Len(t, emails, 1)

		proUserWithoutLastInvoice := console.User{
			ID:              testrand.UUID(),
			FullName:        "Pro User",
			Email:           email + "2",
			Status:          console.UserRequestedDeletion,
			PaidTier:        true,
			PasswordHash:    []byte("password"),
			StatusUpdatedAt: &now,
		}

		_, err = usersRepo.Insert(ctx, &proUserWithoutLastInvoice)
		require.NoError(t, err)

		// Required to set the marked for deletion status.
		err = usersRepo.Update(ctx, proUserWithoutLastInvoice.ID, console.UpdateUserRequest{
			Status: &proUserWithoutLastInvoice.Status,
		})
		require.NoError(t, err)

		emails, err = usersRepo.GetEmailsForDeletion(ctx, nowPlusMinute)
		require.NoError(t, err)
		require.Len(t, emails, 1)

		invoiceGenerated := true
		err = usersRepo.Update(ctx, proUserWithoutLastInvoice.ID, console.UpdateUserRequest{
			FinalInvoiceGenerated: &invoiceGenerated,
		})
		require.NoError(t, err)

		emails, err = usersRepo.GetEmailsForDeletion(ctx, nowPlusMinute)
		require.NoError(t, err)
		require.Len(t, emails, 2)
	})
}

func TestGetExpiredFreeTrialsAfter(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()
		accountFreezeRepo := db.Console().AccountFreezeEvents()

		now := time.Now()
		expired := now.Add(-time.Hour)
		notExpired := now.Add(time.Hour)

		expiredUser, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "expired",
			Email:           email + "1",
			PasswordHash:    []byte("123a123"),
			Status:          console.Active,
			TrialExpiration: &expired,
		})
		require.NoError(t, err)

		_, err = usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "not expired",
			Email:           email + "2",
			PasswordHash:    []byte("123a123"),
			Status:          console.Active,
			TrialExpiration: &notExpired,
		})
		require.NoError(t, err)

		_, err = usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "nil expiry",
			Email:           email + "3",
			PasswordHash:    []byte("123a123"),
			Status:          console.Active,
			TrialExpiration: nil,
		})
		require.NoError(t, err)

		// expect pro user with expired trial to not be returned.
		proUser, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "Paid User",
			Email:           email + "4",
			Status:          console.Active,
			PasswordHash:    []byte("123a123"),
			TrialExpiration: &expired,
		})
		require.NoError(t, err)

		paidTier := true
		err = usersRepo.Update(ctx, proUser.ID, console.UpdateUserRequest{
			PaidTier: &paidTier,
		})
		require.NoError(t, err)

		limit := 100
		users, err := usersRepo.GetExpiredFreeTrialsAfter(ctx, now, limit)
		require.NoError(t, err)
		require.Len(t, users, 1, "expected 1 expired user")
		require.Equal(t, expiredUser.ID, users[0].ID)

		// trial expiration freeze user
		_, err = accountFreezeRepo.Upsert(ctx, &console.AccountFreezeEvent{
			UserID: expiredUser.ID,
			Type:   console.TrialExpirationFreeze,
		})
		require.NoError(t, err)

		users, err = usersRepo.GetExpiredFreeTrialsAfter(ctx, now, limit)
		require.NoError(t, err)
		require.Empty(t, users, "expected no trial frozen users")
	})
}

func TestGetUnverifiedNeedingReminder(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.EmailReminders.FirstVerificationReminder = 24 * time.Hour
				config.EmailReminders.SecondVerificationReminder = 120 * time.Hour
			},
		},
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var sentFirstReminder bool
		var sentSecondReminder bool

		config := planet.Satellites[0].Config.EmailReminders
		db := planet.Satellites[0].DB.Console().Users()

		id := testrand.UUID()
		_, err := db.Insert(ctx, &console.User{
			ID:           id,
			FullName:     "unverified user one",
			Email:        "userone@mail.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		now := time.Now()

		// We expect two reminders in total - one after a day of account creation,
		// and one after five. This test will check to ensure that both reminders occur.
		// Each iteration advances time by i*24 hours from `now`.
		for i := 0; i <= 6; i++ {
			u, err := db.Get(ctx, id)
			require.NoError(t, err)

			// Intuitively it would be better to test this by setting `created_at` to some point in the past.
			// Since we have no control over `created_at` (it's autoinserted) we will instead pass in a future time
			// as the `now` argument to `GetUnverifiedNeedingReminder`
			futureTime := now.Add(time.Duration(i*24) * time.Hour)
			needReminder, err := db.GetUnverifiedNeedingReminder(ctx, futureTime.Add(-config.FirstVerificationReminder), futureTime.Add(-config.SecondVerificationReminder), now.Add(-time.Hour))
			require.NoError(t, err)

			// These are the conditions in the SQL query which selects users needing reminder
			if u.VerificationReminders == 0 && u.CreatedAt.Before(futureTime.Add(-config.FirstVerificationReminder)) {
				require.NotEmpty(t, needReminder)
				require.Equal(t, u.ID, needReminder[0].ID)
				require.NoError(t, db.UpdateVerificationReminders(ctx, u.ID))
				sentFirstReminder = true
			} else if u.VerificationReminders == 1 && u.CreatedAt.Before(futureTime.Add(-config.SecondVerificationReminder)) {
				require.NotEmpty(t, needReminder)
				require.Equal(t, u.ID, needReminder[0].ID)
				require.NoError(t, db.UpdateVerificationReminders(ctx, u.ID))
				sentSecondReminder = true
			} else {
				require.Empty(t, needReminder)
			}
		}
		require.True(t, sentFirstReminder)
		require.True(t, sentSecondReminder)
	})
}

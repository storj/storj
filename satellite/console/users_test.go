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

		// test inserting paid user
		user = &console.User{
			ID:           testrand.UUID(),
			FullName:     name,
			ShortName:    lastName,
			Email:        email,
			Kind:         console.PaidUser,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}
		user, err := repository.Insert(ctx, user)
		assert.NoError(t, err)
		assert.Equal(t, console.PaidUser, user.Kind)
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

			retrievedUser, err := db.Console().Users().GetByEmailAndTenant(ctx, testCase.email, nil)
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
		require.Equal(t, console.FreeUser, createdUser.Kind)

		now := time.Now()
		expiration := now.Add(time.Hour * 24 * 30)
		expirationPtr := &expiration
		notifications := console.TrialExpirationReminder

		err = db.Console().Users().Update(ctx, createdUser.ID, console.UpdateUserRequest{
			TrialNotifications: &notifications,
			TrialExpiration:    &expirationPtr,
		})
		require.NoError(t, err)

		retrievedUser, err := db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedUser.TrialExpiration)
		require.WithinDuration(t, expiration, *retrievedUser.TrialExpiration, time.Minute)
		require.Equal(t, int(notifications), retrievedUser.TrialNotifications)

		err = db.Console().Users().UpdatePaidTier(ctx, createdUser.ID, true, projectBandwidthLimit, storageStorageLimit, segmentLimit, projectLimit, &now)
		require.NoError(t, err)

		retrievedUser, err = db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.Equal(t, email, retrievedUser.Email)
		require.Equal(t, fullName, retrievedUser.FullName)
		require.Equal(t, shortName, retrievedUser.ShortName)
		require.Equal(t, console.PaidUser, retrievedUser.Kind)
		require.WithinDuration(t, now, *retrievedUser.UpgradeTime, time.Minute)
		require.Nil(t, retrievedUser.TrialExpiration)
		require.Zero(t, retrievedUser.TrialNotifications)

		err = db.Console().Users().UpdatePaidTier(ctx, createdUser.ID, false, projectBandwidthLimit, storageStorageLimit, segmentLimit, projectLimit, nil)
		require.NoError(t, err)

		retrievedUser, err = db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, retrievedUser.Kind)
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
		userByEmail, err := repository.GetByEmailAndTenant(ctx, email, nil)
		assert.NoError(t, err)
		assert.Equal(t, name, userByEmail.FullName)
		assert.Equal(t, lastName, userByEmail.ShortName)
		assert.Equal(t, user.SignupPromoCode, userByEmail.SignupPromoCode)
		assert.Equal(t, console.FreeUser, user.Kind)
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
		oldUser, err := repository.GetByEmailAndTenant(ctx, email, nil)
		assert.NoError(t, err)

		newUserInfo := &console.User{
			ID:               oldUser.ID,
			FullName:         newName,
			ShortName:        newLastName,
			Email:            newEmail,
			Status:           console.Active,
			Kind:             console.PaidUser,
			MFAEnabled:       true,
			MFASecretKey:     mfaSecretKey,
			MFARecoveryCodes: []string{"1", "2"},
			PasswordHash:     []byte(newPass),
		}

		shortNamePtr := &newUserInfo.ShortName
		secretKeyPtr := &newUserInfo.MFASecretKey

		year, month, day := time.Now().UTC().Date()
		timestamp := time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
		repository.TestSetNow(func() time.Time { return timestamp })

		err = repository.Update(ctx, newUserInfo.ID, console.UpdateUserRequest{
			FullName:         &newUserInfo.FullName,
			ShortName:        &shortNamePtr,
			Email:            &newUserInfo.Email,
			Status:           &newUserInfo.Status,
			Kind:             &newUserInfo.Kind,
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
		assert.Equal(t, console.PaidUser, newUser.Kind)
		assert.True(t, newUser.MFAEnabled)
		assert.Equal(t, mfaSecretKey, newUser.MFASecretKey)
		assert.Equal(t, newUserInfo.MFARecoveryCodes, newUser.MFARecoveryCodes)
		assert.Equal(t, oldUser.CreatedAt, newUser.CreatedAt)
		assert.NotNil(t, newUser.StatusUpdatedAt)
		assert.WithinDuration(t, timestamp, *newUser.StatusUpdatedAt, time.Minute)
	})

	t.Run("Delete user success", func(t *testing.T) {
		oldUser, err := repository.GetByEmailAndTenant(ctx, newEmail, nil)
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

		_, err = usersRepo.GetByEmailAndTenant(ctx, email, nil)
		require.ErrorIs(t, sql.ErrNoRows, err)

		verified, unverified, err := usersRepo.GetByEmailAndTenantWithUnverified(ctx, email, nil)
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

		dbUser, err := usersRepo.GetByEmailAndTenant(ctx, email, nil)
		require.NoError(t, err)
		require.Equal(t, activeUser.ID, dbUser.ID)

		tenantID := "test-tenant"
		tenantUserEmail := email + "tenant"
		tenantUser := console.User{
			ID:           testrand.UUID(),
			FullName:     "Tenant User",
			Email:        tenantUserEmail,
			PasswordHash: []byte("password"),
			Status:       console.Active,
			TenantID:     &tenantID,
		}
		_, err = usersRepo.Insert(ctx, &tenantUser)
		require.NoError(t, err)

		err = usersRepo.Update(ctx, tenantUser.ID, console.UpdateUserRequest{
			Status: &tenantUser.Status,
		})
		require.NoError(t, err)

		dbUser, err = usersRepo.GetByEmailAndTenant(ctx, tenantUserEmail, &tenantID)
		require.NoError(t, err)
		require.Equal(t, tenantUser.ID, dbUser.ID)
		require.Equal(t, tenantUser.TenantID, dbUser.TenantID)

		verified, unverified, err = usersRepo.GetByEmailAndTenantWithUnverified(ctx, tenantUserEmail, &tenantID)
		require.NoError(t, err)
		require.Nil(t, unverified)
		require.Equal(t, tenantUser.ID, verified.ID)
		require.Equal(t, tenantUser.TenantID, verified.TenantID)
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
			ID:           testrand.UUID(),
			FullName:     "Free Trial User",
			Email:        email + "1",
			Status:       console.UserRequestedDeletion,
			PasswordHash: []byte("password"),
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
			ID:           testrand.UUID(),
			FullName:     "Pro User",
			Email:        email + "2",
			Status:       console.UserRequestedDeletion,
			Kind:         console.PaidUser,
			PasswordHash: []byte("password"),
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

func TestGetUserInfoByProjectID(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		projects := db.Console().Projects()
		users := db.Console().Users()

		user, err := users.Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Test user",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		active := console.Active
		err = users.Update(ctx, user.ID, console.UpdateUserRequest{Status: &active})
		require.NoError(t, err)

		prj, err := projects.Insert(ctx, &console.Project{
			Name:        "ProjectName",
			Description: "projects description",
			OwnerID:     user.ID,
		})
		require.NoError(t, err)

		info, err := users.GetUserInfoByProjectID(ctx, prj.ID)
		require.NoError(t, err)
		require.Equal(t, active, info.Status)

		pendingDeletion := console.PendingDeletion
		err = users.Update(ctx, user.ID, console.UpdateUserRequest{Status: &pendingDeletion})
		require.NoError(t, err)

		info, err = users.GetUserInfoByProjectID(ctx, prj.ID)
		require.NoError(t, err)
		require.Equal(t, pendingDeletion, info.Status)
	})
}

func TestGetExpiredFreeTrialsAfter(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		usersRepo := db.Console().Users()
		accountFreezeRepo := db.Console().AccountFreezeEvents()

		now := time.Now()
		expired := now.Add(-time.Hour)
		notExpired := now.Add(time.Hour)
		activeStatus := console.Active

		expiredUser, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "expired",
			Email:           email + "1",
			PasswordHash:    []byte("123a123"),
			TrialExpiration: &expired,
		})
		require.NoError(t, err)

		err = usersRepo.Update(ctx, expiredUser.ID, console.UpdateUserRequest{Status: &activeStatus})
		require.NoError(t, err)

		notExpiredUser, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "not expired",
			Email:           email + "2",
			PasswordHash:    []byte("123a123"),
			TrialExpiration: &notExpired,
		})
		require.NoError(t, err)

		err = usersRepo.Update(ctx, notExpiredUser.ID, console.UpdateUserRequest{Status: &activeStatus})
		require.NoError(t, err)

		notExpiredUser1, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "nil expiry",
			Email:           email + "3",
			PasswordHash:    []byte("123a123"),
			TrialExpiration: nil,
		})
		require.NoError(t, err)

		err = usersRepo.Update(ctx, notExpiredUser1.ID, console.UpdateUserRequest{Status: &activeStatus})
		require.NoError(t, err)

		// expect pro user with expired trial to not be returned.
		proUser, err := usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "Paid User",
			Email:           email + "4",
			PasswordHash:    []byte("123a123"),
			TrialExpiration: &expired,
		})
		require.NoError(t, err)

		err = usersRepo.Update(ctx, proUser.ID, console.UpdateUserRequest{Status: &activeStatus})
		require.NoError(t, err)

		// expect inactive user with expired trial to not be returned.
		_, err = usersRepo.Insert(ctx, &console.User{
			ID:              testrand.UUID(),
			FullName:        "expired user",
			Email:           email + "5",
			PasswordHash:    []byte("123a123"),
			TrialExpiration: &expired,
		})
		require.NoError(t, err)

		kind := console.PaidUser
		err = usersRepo.Update(ctx, proUser.ID, console.UpdateUserRequest{
			Kind: &kind,
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
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.EmailReminders.FirstVerificationReminder = 24 * time.Hour
				config.EmailReminders.SecondVerificationReminder = 120 * time.Hour
			},
		},
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

func TestUserStatus(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		for i := 0; i < console.UserStatusCount; i++ {
			status := console.UserStatus(i)
			require.NotEmptyf(
				t, status.String(), "status without associated string representation: %d", i,
			)
		}

		// We add one to the highest value to verify it returns an empty string.
		status := console.UserStatus(console.UserStatusCount)
		require.Emptyf(t,
			status.String(),
			"invalid status should return empty string: %d", console.UserStatusCount,
		)
	})

	t.Run("Set", func(t *testing.T) {
		tcases := []struct {
			status   string
			isValid  bool
			expected console.UserStatus
		}{
			{
				status:   "inactive",
				isValid:  true,
				expected: console.Inactive,
			},
			{
				status:   "Active",
				isValid:  true,
				expected: console.Active,
			},
			{
				status:   "DELETED",
				isValid:  true,
				expected: console.Deleted,
			},
			{
				status:   "PendinG DeletioN",
				isValid:  true,
				expected: console.PendingDeletion,
			},
			{
				status:   "Legal Hold",
				isValid:  true,
				expected: console.LegalHold,
			},
			{
				status:   "pending bot verification",
				isValid:  true,
				expected: console.PendingBotVerification,
			},
			{
				status:   "user requested Deletion",
				isValid:  true,
				expected: console.UserRequestedDeletion,
			},
			{
				status:  "does not exists this status",
				isValid: false,
			},
		}

		var status console.UserStatus
		for _, tcase := range tcases {
			err := status.Set(tcase.status)
			if err != nil {
				require.False(t, tcase.isValid)
				require.ErrorContains(t, err, tcase.status)
			} else {
				require.True(t, tcase.isValid)
				require.NoError(t, err)
				require.Equal(t, tcase.expected, status)
			}
		}
	})
}

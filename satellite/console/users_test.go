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

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const (
	lastName        = "lastName"
	email           = "email@mail.test"
	passValid       = "123456"
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
		partnerID := testrand.UUID()

		// Test with and without partnerID
		user := &console.User{
			ID:           testrand.UUID(),
			FullName:     name,
			ShortName:    lastName,
			Email:        email,
			PartnerID:    partnerID,
			PasswordHash: []byte(passValid),
			CreatedAt:    time.Now(),
		}
		testUsers(ctx, t, repository, user)

		user = &console.User{
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
			{email: "prettyandsimple@example.com"},
			{email: "firstname.lastname@domain.com	"},
			{email: "email@subdomain.domain.com	"},
			{email: "firstname+lastname@domain.com	"},
			{email: "email@[123.123.123.123]	"},
			{email: "\"email\"@domain.com"},
			{email: "_______@domain.com	"},
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

			err = db.Console().Users().Update(ctx, createdUser)
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

		err = db.Console().Users().UpdatePaidTier(ctx, createdUser.ID, true, projectBandwidthLimit, storageStorageLimit, segmentLimit, projectLimit)
		require.NoError(t, err)

		retrievedUser, err := db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.Equal(t, email, retrievedUser.Email)
		require.Equal(t, fullName, retrievedUser.FullName)
		require.Equal(t, shortName, retrievedUser.ShortName)
		require.True(t, retrievedUser.PaidTier)

		err = db.Console().Users().UpdatePaidTier(ctx, createdUser.ID, false, projectBandwidthLimit, storageStorageLimit, segmentLimit, projectLimit)
		require.NoError(t, err)

		retrievedUser, err = db.Console().Users().Get(ctx, createdUser.ID)
		require.NoError(t, err)
		require.False(t, retrievedUser.PaidTier)
	})
}

func testUsers(ctx context.Context, t *testing.T, repository console.Users, user *console.User) {

	t.Run("User insertion success", func(t *testing.T) {

		insertedUser, err := repository.Insert(ctx, user)
		assert.NoError(t, err)

		insertedUser.Status = console.Active

		err = repository.Update(ctx, insertedUser)
		assert.NoError(t, err)
	})

	t.Run("Get user success", func(t *testing.T) {
		userByEmail, err := repository.GetByEmail(ctx, email)
		assert.NoError(t, err)
		assert.Equal(t, name, userByEmail.FullName)
		assert.Equal(t, lastName, userByEmail.ShortName)
		assert.Equal(t, user.PartnerID, userByEmail.PartnerID)
		assert.Equal(t, user.SignupPromoCode, userByEmail.SignupPromoCode)
		assert.False(t, user.PaidTier)
		assert.False(t, user.MFAEnabled)
		assert.Empty(t, user.MFASecretKey)
		assert.Empty(t, user.MFARecoveryCodes)
		assert.Empty(t, user.LastVerificationReminder)

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
		assert.Equal(t, user.PartnerID, userByID.PartnerID)
		assert.Equal(t, user.SignupPromoCode, userByID.SignupPromoCode)
		assert.False(t, user.MFAEnabled)
		assert.Empty(t, user.MFASecretKey)
		assert.Empty(t, user.MFARecoveryCodes)
		assert.Empty(t, user.LastVerificationReminder)

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
		assert.Equal(t, userByID.PartnerID, userByEmail.PartnerID)
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

		d := (60 * time.Second)
		date := time.Now().Add(-24 * 365 * time.Hour).Truncate(d)

		newUserInfo := &console.User{
			ID:                       oldUser.ID,
			FullName:                 newName,
			ShortName:                newLastName,
			Email:                    newEmail,
			Status:                   console.Active,
			PaidTier:                 true,
			MFAEnabled:               true,
			MFASecretKey:             mfaSecretKey,
			MFARecoveryCodes:         []string{"1", "2"},
			PasswordHash:             []byte(newPass),
			LastVerificationReminder: date,
		}

		err = repository.Update(ctx, newUserInfo)
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
		assert.Equal(t, newUserInfo.LastVerificationReminder, newUser.LastVerificationReminder)
		// PartnerID should not change
		assert.Equal(t, user.PartnerID, newUser.PartnerID)
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
			PasswordHash: []byte("123a123"),
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
			PasswordHash: []byte("123a123"),
		}

		_, err = usersRepo.Insert(ctx, &activeUser)
		require.NoError(t, err)

		// Required to set the active status.
		err = usersRepo.Update(ctx, &activeUser)
		require.NoError(t, err)

		dbUser, err := usersRepo.GetByEmail(ctx, email)
		require.NoError(t, err)
		require.Equal(t, activeUser.ID, dbUser.ID)
	})
}

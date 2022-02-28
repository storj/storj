// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that users implements console.Users.
var _ console.Users = (*users)(nil)

// implementation of Users interface repository using spacemonkeygo/dbx orm.
type users struct {
	db dbx.Methods
}

// Get is a method for querying user from the database by id.
func (users *users) Get(ctx context.Context, id uuid.UUID) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := users.db.Get_User_By_Id(ctx, dbx.User_Id(id[:]))

	if err != nil {
		return nil, err
	}

	return userFromDBX(ctx, user)
}

// GetByEmailWithUnverified is a method for querying users by email from the database.
func (users *users) GetByEmailWithUnverified(ctx context.Context, email string) (verified *console.User, unverified []console.User, err error) {
	defer mon.Task()(&ctx)(&err)
	usersDbx, err := users.db.All_User_By_NormalizedEmail(ctx, dbx.User_NormalizedEmail(normalizeEmail(email)))

	if err != nil {
		return nil, nil, err
	}

	var errors errs.Group
	for _, userDbx := range usersDbx {
		u, err := userFromDBX(ctx, userDbx)
		if err != nil {
			errors.Add(err)
			continue
		}

		if u.Status == console.Active {
			verified = u
		} else {
			unverified = append(unverified, *u)
		}
	}

	return verified, unverified, errors.Err()
}

// GetByEmail is a method for querying user by verified email from the database.
func (users *users) GetByEmail(ctx context.Context, email string) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := users.db.Get_User_By_NormalizedEmail_And_Status_Not_Number(ctx, dbx.User_NormalizedEmail(normalizeEmail(email)))

	if err != nil {
		return nil, err
	}

	return userFromDBX(ctx, user)
}

// Insert is a method for inserting user into the database.
func (users *users) Insert(ctx context.Context, user *console.User) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	if user.ID.IsZero() {
		return nil, errs.New("user id is not set")
	}

	optional := dbx.User_Create_Fields{
		ShortName:       dbx.User_ShortName(user.ShortName),
		IsProfessional:  dbx.User_IsProfessional(user.IsProfessional),
		SignupPromoCode: dbx.User_SignupPromoCode(user.SignupPromoCode),
	}
	if !user.PartnerID.IsZero() {
		optional.PartnerId = dbx.User_PartnerId(user.PartnerID[:])
	}
	if user.UserAgent != nil {
		optional.UserAgent = dbx.User_UserAgent(user.UserAgent)
	}
	if user.ProjectLimit != 0 {
		optional.ProjectLimit = dbx.User_ProjectLimit(user.ProjectLimit)
	}
	if user.ProjectStorageLimit != 0 {
		optional.ProjectStorageLimit = dbx.User_ProjectStorageLimit(user.ProjectStorageLimit)
	}
	if user.ProjectBandwidthLimit != 0 {
		optional.ProjectBandwidthLimit = dbx.User_ProjectBandwidthLimit(user.ProjectBandwidthLimit)
	}
	if user.ProjectSegmentLimit != 0 {
		optional.ProjectSegmentLimit = dbx.User_ProjectSegmentLimit(user.ProjectSegmentLimit)
	}
	if user.IsProfessional {
		optional.Position = dbx.User_Position(user.Position)
		optional.CompanyName = dbx.User_CompanyName(user.CompanyName)
		optional.WorkingOn = dbx.User_WorkingOn(user.WorkingOn)
		optional.EmployeeCount = dbx.User_EmployeeCount(user.EmployeeCount)
		optional.HaveSalesContact = dbx.User_HaveSalesContact(user.HaveSalesContact)
	}

	createdUser, err := users.db.Create_User(ctx,
		dbx.User_Id(user.ID[:]),
		dbx.User_Email(user.Email),
		dbx.User_NormalizedEmail(normalizeEmail(user.Email)),
		dbx.User_FullName(user.FullName),
		dbx.User_PasswordHash(user.PasswordHash),
		optional,
	)

	if err != nil {
		return nil, err
	}

	return userFromDBX(ctx, createdUser)
}

// Delete is a method for deleting user by Id from the database.
func (users *users) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = users.db.Delete_User_By_Id(ctx, dbx.User_Id(id[:]))

	return err
}

// Update is a method for updating user entity.
func (users *users) Update(ctx context.Context, user *console.User) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields, err := toUpdateUser(user)
	if err != nil {
		return err
	}

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(user.ID[:]),
		*updateFields,
	)

	return err
}

// UpdatePaidTier sets whether the user is in the paid tier.
func (users *users) UpdatePaidTier(ctx context.Context, id uuid.UUID, paidTier bool, projectBandwidthLimit, projectStorageLimit memory.Size, projectSegmentLimit int64, projectLimit int) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(id[:]),
		dbx.User_Update_Fields{
			PaidTier:              dbx.User_PaidTier(paidTier),
			ProjectLimit:          dbx.User_ProjectLimit(projectLimit),
			ProjectBandwidthLimit: dbx.User_ProjectBandwidthLimit(projectBandwidthLimit.Int64()),
			ProjectStorageLimit:   dbx.User_ProjectStorageLimit(projectStorageLimit.Int64()),
			ProjectSegmentLimit:   dbx.User_ProjectSegmentLimit(projectSegmentLimit),
		},
	)

	return err
}

// GetProjectLimit is a method to get the users project limit.
func (users *users) GetProjectLimit(ctx context.Context, id uuid.UUID) (limit int, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := users.db.Get_User_ProjectLimit_By_Id(ctx, dbx.User_Id(id[:]))
	if err != nil {
		return 0, err
	}
	return row.ProjectLimit, nil
}

// GetUserProjectLimits is a method to get the users storage and bandwidth limits for new projects.
func (users *users) GetUserProjectLimits(ctx context.Context, id uuid.UUID) (limits *console.ProjectLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := users.db.Get_User_ProjectStorageLimit_User_ProjectBandwidthLimit_User_ProjectSegmentLimit_By_Id(ctx, dbx.User_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return limitsFromDBX(ctx, row)
}

func (users *users) GetUserPaidTier(ctx context.Context, id uuid.UUID) (isPaid bool, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := users.db.Get_User_PaidTier_By_Id(ctx, dbx.User_Id(id[:]))
	if err != nil {
		return false, err
	}
	return row.PaidTier, nil
}

// toUpdateUser creates dbx.User_Update_Fields with only non-empty fields as updatable.
func toUpdateUser(user *console.User) (*dbx.User_Update_Fields, error) {
	update := dbx.User_Update_Fields{
		FullName:              dbx.User_FullName(user.FullName),
		ShortName:             dbx.User_ShortName(user.ShortName),
		Email:                 dbx.User_Email(user.Email),
		NormalizedEmail:       dbx.User_NormalizedEmail(normalizeEmail(user.Email)),
		Status:                dbx.User_Status(int(user.Status)),
		ProjectLimit:          dbx.User_ProjectLimit(user.ProjectLimit),
		ProjectStorageLimit:   dbx.User_ProjectStorageLimit(user.ProjectStorageLimit),
		ProjectBandwidthLimit: dbx.User_ProjectBandwidthLimit(user.ProjectBandwidthLimit),
		ProjectSegmentLimit:   dbx.User_ProjectSegmentLimit(user.ProjectSegmentLimit),
		PaidTier:              dbx.User_PaidTier(user.PaidTier),
		MfaEnabled:            dbx.User_MfaEnabled(user.MFAEnabled),
	}

	recoveryBytes, err := json.Marshal(user.MFARecoveryCodes)
	if err != nil {
		return nil, err
	}
	update.MfaRecoveryCodes = dbx.User_MfaRecoveryCodes(string(recoveryBytes))
	update.MfaSecretKey = dbx.User_MfaSecretKey(user.MFASecretKey)
	update.LastVerificationReminder = dbx.User_LastVerificationReminder(user.LastVerificationReminder)

	// extra password check to update only calculated hash from service
	if len(user.PasswordHash) != 0 {
		update.PasswordHash = dbx.User_PasswordHash(user.PasswordHash)
	}

	return &update, nil
}

// userFromDBX is used for creating User entity from autogenerated dbx.User struct.
func userFromDBX(ctx context.Context, user *dbx.User) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)
	if user == nil {
		return nil, errs.New("user parameter is nil")
	}

	id, err := uuid.FromBytes(user.Id)
	if err != nil {
		return nil, err
	}

	var recoveryCodes []string
	if user.MfaRecoveryCodes != nil {
		err = json.Unmarshal([]byte(*user.MfaRecoveryCodes), &recoveryCodes)
		if err != nil {
			return nil, err
		}
	}

	result := console.User{
		ID:                    id,
		FullName:              user.FullName,
		Email:                 user.Email,
		PasswordHash:          user.PasswordHash,
		Status:                console.UserStatus(user.Status),
		CreatedAt:             user.CreatedAt,
		ProjectLimit:          user.ProjectLimit,
		ProjectBandwidthLimit: user.ProjectBandwidthLimit,
		ProjectStorageLimit:   user.ProjectStorageLimit,
		ProjectSegmentLimit:   user.ProjectSegmentLimit,
		PaidTier:              user.PaidTier,
		IsProfessional:        user.IsProfessional,
		HaveSalesContact:      user.HaveSalesContact,
		MFAEnabled:            user.MfaEnabled,
	}

	if user.PartnerId != nil {
		result.PartnerID, err = uuid.FromBytes(user.PartnerId)
		if err != nil {
			return nil, err
		}
	}

	if user.UserAgent != nil {
		result.UserAgent = user.UserAgent
	}

	if user.ShortName != nil {
		result.ShortName = *user.ShortName
	}

	if user.Position != nil {
		result.Position = *user.Position
	}

	if user.CompanyName != nil {
		result.CompanyName = *user.CompanyName
	}

	if user.WorkingOn != nil {
		result.WorkingOn = *user.WorkingOn
	}

	if user.EmployeeCount != nil {
		result.EmployeeCount = *user.EmployeeCount
	}

	if user.MfaSecretKey != nil {
		result.MFASecretKey = *user.MfaSecretKey
	}

	if user.MfaRecoveryCodes != nil {
		result.MFARecoveryCodes = recoveryCodes
	}

	if user.SignupPromoCode != nil {
		result.SignupPromoCode = *user.SignupPromoCode
	}

	if user.LastVerificationReminder != nil {
		result.LastVerificationReminder = *user.LastVerificationReminder
	}

	return &result, nil
}

// limitsFromDBX is used for creating user project limits entity from autogenerated dbx.User struct.
func limitsFromDBX(ctx context.Context, limits *dbx.ProjectStorageLimit_ProjectBandwidthLimit_ProjectSegmentLimit_Row) (_ *console.ProjectLimits, err error) {
	defer mon.Task()(&ctx)(&err)
	if limits == nil {
		return nil, errs.New("user parameter is nil")
	}

	result := console.ProjectLimits{
		ProjectBandwidthLimit: memory.Size(limits.ProjectBandwidthLimit),
		ProjectStorageLimit:   memory.Size(limits.ProjectStorageLimit),
		ProjectSegmentLimit:   limits.ProjectSegmentLimit,
	}
	return &result, nil
}

func normalizeEmail(email string) string {
	return strings.ToUpper(email)
}

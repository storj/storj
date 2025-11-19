// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
)

// ensures that users implements console.Users.
var _ console.Users = (*users)(nil)

// implementation of Users interface repository using spacemonkeygo/dbx orm.
type users struct {
	db   dbx.DriverMethods
	impl dbutil.Implementation

	nowFn func() time.Time
}

// UpdateFailedLoginCountAndExpiration increments failed_login_count and sets login_lockout_expiration appropriately.
func (users *users) UpdateFailedLoginCountAndExpiration(ctx context.Context, failedLoginPenalty *float64, id uuid.UUID, now time.Time) (err error) {
	if failedLoginPenalty != nil {
		// failed_login_count exceeded config.FailedLoginPenalty
		switch users.impl {
		case dbutil.Postgres, dbutil.Cockroach:
			_, err = users.db.ExecContext(ctx, users.db.Rebind(`
			UPDATE users
			SET failed_login_count = COALESCE(failed_login_count, 0) + 1,
				login_lockout_expiration = ?::TIMESTAMPTZ + POWER(?, failed_login_count-1) * INTERVAL '1 minute'
			WHERE id = ?
		`), now, failedLoginPenalty, id.Bytes())
		case dbutil.Spanner:
			_, err = users.db.ExecContext(ctx, users.db.Rebind(`
			UPDATE users
			SET failed_login_count = IFNULL(failed_login_count, 0) + 1,
				login_lockout_expiration = TIMESTAMP_ADD(?, INTERVAL CAST(POW(?, failed_login_count - 1) AS INT64) MINUTE)
			WHERE id = ?
		`), now, failedLoginPenalty, id.Bytes())
		default:
			return errs.New("unsupported database dialect: %s", users.impl)
		}
	} else {
		_, err = users.db.ExecContext(ctx, users.db.Rebind(`
			UPDATE users
			SET failed_login_count = COALESCE(failed_login_count, 0) + 1
			WHERE id = ?
		`), id.Bytes())
	}
	return
}

// Search searches for users by a search term in their name or email.
// Results are limited to 100 users.
func (users *users) Search(ctx context.Context, term string) (_ []console.UserInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	search := "%" + strings.ReplaceAll(term, " ", "%") + "%"
	query := `
		SELECT id, full_name, email, status, kind, created_at
		FROM users
		WHERE normalized_email LIKE UPPER(?)
		   OR LOWER(full_name) LIKE LOWER(?)
		ORDER BY normalized_email ASC
		LIMIT 100;`

	rows, err := users.db.QueryContext(ctx, users.db.Rebind(query), search, search)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

	var userInfos []console.UserInfo
	for rows.Next() {
		var usr console.UserInfo
		if err := rows.Scan(&usr.ID, &usr.FullName, &usr.Email, &usr.Status, &usr.Kind, &usr.CreatedAt); err != nil {
			return nil, err
		}
		userInfos = append(userInfos, usr)
	}

	return userInfos, nil
}

// Get is a method for querying user from the database by id.
func (users *users) Get(ctx context.Context, id uuid.UUID) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := users.db.Get_User_By_Id(ctx, dbx.User_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return UserFromDBX(ctx, user)
}

// GetByCustomerID returns the user with the given customer ID.
func (users *users) GetByCustomerID(ctx context.Context, customerID string) (_ *console.UserInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT u.id, u.full_name, u.email, u.status, u.kind, u.created_at
		FROM users AS u
		WHERE u.id = (SELECT user_id FROM stripe_customers WHERE customer_id = ?);
	`
	row := users.db.QueryRowContext(ctx, users.db.Rebind(query), customerID)
	if row.Err() != nil {
		return nil, Error.Wrap(row.Err())
	}
	var usr console.UserInfo
	err = row.Scan(&usr.ID, &usr.FullName, &usr.Email, &usr.Status, &usr.Kind, &usr.CreatedAt)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &usr, nil
}

// GetExpiredFreeTrialsAfter is a method for querying users that are in free trial from the database with trial expiry (after)
// AND have not been frozen.
func (users *users) GetExpiredFreeTrialsAfter(ctx context.Context, after time.Time, limit int) ([]console.User, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if limit == 0 {
		return nil, Error.New("limit cannot be 0")
	}

	rows, err := users.db.QueryContext(ctx, users.db.Rebind(`
		SELECT u.id, u.email FROM users AS u
		LEFT JOIN account_freeze_events AS ae
			ON u.id = ae.user_id
		WHERE u.kind = ?
			AND u.trial_expiration < ?
			AND u.status > ?
			AND ae.user_id IS NULL
		LIMIT ?;`), console.FreeUser, after, console.Inactive, limit)
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return []console.User{}, nil
		}
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	expiredUsers := make([]console.User, 0, limit)
	for rows.Next() {
		var user console.User
		err = rows.Scan(&user.ID, &user.Email)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		expiredUsers = append(expiredUsers, user)
	}

	return expiredUsers, Error.Wrap(rows.Err())
}

// GetByEmailAndTenantWithUnverified is a method for querying users by email and tenantID from the database.
func (users *users) GetByEmailAndTenantWithUnverified(ctx context.Context, email string, tenantID *string) (verified *console.User, unverified []console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	var dbxTenantID dbx.User_TenantId_Field
	if tenantID == nil || *tenantID == "" {
		dbxTenantID = dbx.User_TenantId_Null()
	} else {
		dbxTenantID = dbx.User_TenantId(*tenantID)
	}

	usersDbx, err := users.db.All_User_By_NormalizedEmail_And_TenantId(ctx, dbx.User_NormalizedEmail(normalizeEmail(email)), dbxTenantID)

	if err != nil {
		return nil, nil, err
	}

	var errors errs.Group
	for _, userDbx := range usersDbx {
		u, err := UserFromDBX(ctx, userDbx)
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

// GetByExternalID is a method for querying user by external ID from the database.
func (users *users) GetByExternalID(ctx context.Context, externalID string) (user *console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	userDbx, err := users.db.Get_User_By_ExternalId(ctx, dbx.User_ExternalId(externalID))
	if err != nil {
		return nil, err
	}

	u, err := UserFromDBX(ctx, userDbx)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (users *users) GetByStatus(ctx context.Context, status console.UserStatus, cursor console.UserCursor) (page *console.UsersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	if cursor.Limit == 0 {
		return nil, Error.New("limit cannot be 0")
	}

	if cursor.Page == 0 {
		return nil, Error.New("page cannot be 0")
	}

	page = &console.UsersPage{
		Limit:  cursor.Limit,
		Offset: uint64((cursor.Page - 1) * cursor.Limit),
	}

	count, err := users.db.Count_User_By_Status(ctx, dbx.User_Status(int(status)))
	if err != nil {
		return nil, err
	}
	page.TotalCount = uint64(count)

	if page.TotalCount == 0 {
		return page, nil
	}
	if page.Offset > page.TotalCount-1 {
		return nil, Error.New("page is out of range")
	}

	dbxUsers, err := users.db.Limited_User_Id_User_Email_User_FullName_By_Status(ctx,
		dbx.User_Status(int(status)),
		int(page.Limit), int64(page.Offset))
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return &console.UsersPage{
				Users: []console.User{},
			}, nil
		}
		return nil, Error.Wrap(err)
	}

	for _, usr := range dbxUsers {
		id, err := uuid.FromBytes(usr.Id)
		if err != nil {
			return &console.UsersPage{
				Users: []console.User{},
			}, nil
		}
		page.Users = append(page.Users, console.User{
			ID:       id,
			Email:    usr.Email,
			FullName: usr.FullName,
		})
	}

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.CurrentPage = cursor.Page

	return page, nil
}

// GetUserInfoByProjectID gets the user info of the project (id) owner.
func (users *users) GetUserInfoByProjectID(ctx context.Context, id uuid.UUID) (_ *console.UserInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	statusRow, err := users.db.Get_User_Status_By_Project_Id(ctx, dbx.Project_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return &console.UserInfo{
		Status: console.UserStatus(statusRow.Status),
	}, nil
}

// GetByEmailAndTenant is a method for querying user by email and tenantID from the database.
func (users *users) GetByEmailAndTenant(ctx context.Context, email string, tenantID *string) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	var dbxTenantID dbx.User_TenantId_Field
	if tenantID == nil || *tenantID == "" {
		dbxTenantID = dbx.User_TenantId_Null()
	} else {
		dbxTenantID = dbx.User_TenantId(*tenantID)
	}

	user, err := users.db.Get_User_By_NormalizedEmail_And_TenantId_And_Status_Not_Number(ctx, dbx.User_NormalizedEmail(normalizeEmail(email)), dbxTenantID)

	if err != nil {
		return nil, err
	}

	return UserFromDBX(ctx, user)
}

// GetExpiresBeforeWithStatus returns users with a particular trial notification status and whose trial expires before 'expiresBefore'.
func (users *users) GetExpiresBeforeWithStatus(ctx context.Context, notificationStatus console.TrialNotificationStatus, expiresBefore time.Time) (needNotification []*console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := users.db.QueryContext(ctx, users.db.Rebind(`
		SELECT id, email
		FROM users
		WHERE kind = ?
			AND trial_notifications = ?
			AND trial_expiration < ?
	`), console.FreeUser, notificationStatus, expiresBefore)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var user console.User
		err = rows.Scan(&user.ID, &user.Email)
		if err != nil {
			return nil, err
		}
		needNotification = append(needNotification, &user)
	}

	return needNotification, rows.Err()
}

// GetEmailsForDeletion is a method for querying user emails which were requested for deletion by the user and can be deleted.
func (users *users) GetEmailsForDeletion(ctx context.Context, statusUpdatedBefore time.Time) (emails []string, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := users.db.QueryContext(ctx, users.db.Rebind(`
		SELECT email
		FROM users
		WHERE status = ?
			AND status_updated_at < ?
			AND (kind = ? OR final_invoice_generated = true)
	`), console.UserRequestedDeletion, statusUpdatedBefore, console.FreeUser)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var email string

		err = rows.Scan(&email)
		if err != nil {
			return nil, err
		}

		emails = append(emails, email)
	}

	return emails, rows.Err()
}

// GetUnverifiedNeedingReminder returns users in need of a reminder to verify their email.
func (users *users) GetUnverifiedNeedingReminder(ctx context.Context, firstReminder, secondReminder, cutoff time.Time) (usersNeedingReminder []*console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := users.db.QueryContext(ctx, users.db.Rebind(`
		SELECT id, email, full_name, short_name
		FROM users
		WHERE status = 0
			AND created_at > ?
			AND (
				(verification_reminders = 0 AND created_at < ?)
				OR (verification_reminders = 1 AND created_at < ?)
			)
	`), cutoff, firstReminder, secondReminder)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var user console.User
		err = rows.Scan(&user.ID, &user.Email, &user.FullName, &user.ShortName)
		if err != nil {
			return nil, err
		}
		usersNeedingReminder = append(usersNeedingReminder, &user)
	}

	return usersNeedingReminder, rows.Err()
}

// UpdateVerificationReminders increments verification_reminders.
func (users *users) UpdateVerificationReminders(ctx context.Context, id uuid.UUID) error {
	_, err := users.db.ExecContext(ctx, users.db.Rebind(`
		UPDATE users
		SET verification_reminders = verification_reminders + 1
		WHERE id = ?
	`), id.Bytes())
	return err
}

// Insert is a method for inserting user into the database.
//
// It always insert the user fields ID, Email, FullName and PasswordHash. The ID cannot be zero.
// The rest of the fields are optional.
//
// NOTE this method ignores the user fields: CreatedAt, Status, FinalInvoiceGenerated, MFAEnabled,
// MFASecretKey, MfaRecoveryCodes, VerificationReminders, TrialNotifications, FailedLoginCount,
// LoginLockoutExpiration, UpgradeTime, NewUnverifiedEmail, EmailChangeVerificationStep,
// HubspotObjectID.
func (users *users) Insert(ctx context.Context, user *console.User) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	if user.ID.IsZero() {
		return nil, errs.New("user id is not set")
	}

	optional := dbx.User_Create_Fields{
		ShortName:       dbx.User_ShortName(user.ShortName),
		IsProfessional:  dbx.User_IsProfessional(user.IsProfessional),
		SignupPromoCode: dbx.User_SignupPromoCode(user.SignupPromoCode),
		Kind:            dbx.User_Kind(int(user.Kind)),
	}
	if user.ExternalID != nil {
		optional.ExternalId = dbx.User_ExternalId(*user.ExternalID)
	}
	if user.TenantID != nil && *user.TenantID != "" {
		optional.TenantId = dbx.User_TenantId(*user.TenantID)
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
	if user.SignupCaptcha != nil {
		optional.SignupCaptcha = dbx.User_SignupCaptcha(*user.SignupCaptcha)
	}

	if user.DefaultPlacement > 0 {
		optional.DefaultPlacement = dbx.User_DefaultPlacement(int(user.DefaultPlacement))
	}

	if user.ActivationCode != "" {
		optional.ActivationCode = dbx.User_ActivationCode(user.ActivationCode)
	}

	if user.SignupId != "" {
		optional.SignupId = dbx.User_SignupId(user.SignupId)
	}

	if user.TrialExpiration != nil {
		optional.TrialExpiration = dbx.User_TrialExpiration(*user.TrialExpiration)
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

	return UserFromDBX(ctx, createdUser)
}

// Delete is a method for deleting user by ID from the database.
func (users *users) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = users.db.Delete_User_By_Id(ctx, dbx.User_Id(id[:]))

	return err
}

// DeleteUnverifiedBefore deletes unverified users created prior to some time from the database.
func (users *users) DeleteUnverifiedBefore(
	ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration, pageSize int,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pageSize <= 0 {
		return Error.New("expected page size to be positive; got %d", pageSize)
	}

	var pageCursor uuid.UUID
	selected := make([]uuid.UUID, pageSize)
	aost := users.db.AsOfSystemInterval(asOfSystemTimeInterval)
	for {
		// Select the ID beginning this page of records
		err = users.db.QueryRowContext(ctx, users.db.Rebind(`
			SELECT id FROM users
			`+aost+`
			WHERE id > ? AND users.status = ? AND users.created_at < ?
			ORDER BY id LIMIT 1
		`), pageCursor, console.Inactive, before).Scan(&pageCursor)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		// Select page of records
		rows, err := users.db.QueryContext(ctx, users.db.Rebind(`
			SELECT id FROM users
			`+aost+`
			WHERE id >= ? ORDER BY id LIMIT ?
		`), pageCursor, pageSize)
		if err != nil {
			return Error.Wrap(err)
		}

		var i int
		for i = 0; rows.Next(); i++ {
			if err = rows.Scan(&selected[i]); err != nil {
				return Error.Wrap(err)
			}
		}
		if err = errs.Combine(rows.Err(), rows.Close()); err != nil {
			return Error.Wrap(err)
		}

		switch users.impl {
		case dbutil.Postgres, dbutil.Cockroach:
			// Delete all old, unverified users in the page
			_, err = users.db.ExecContext(ctx, `
			DELETE FROM users
			WHERE id = ANY($1)
			AND status = $2 AND created_at < $3
		`, pgutil.UUIDArray(selected[:i]), console.Inactive, before)
		case dbutil.Spanner:
			// Delete all old, unverified users in the page
			_, err = users.db.ExecContext(ctx, `
			DELETE FROM users
			WHERE id IN UNNEST(?)
			AND status = ? AND created_at < ?
		`, uuidsToBytesArray(selected[:i]), console.Inactive, before)
		default:
			return errs.New("unsupported database dialect: %s", users.impl)
		}
		if err != nil {
			return Error.Wrap(err)
		}

		if i < pageSize {
			return nil
		}

		// Advance the cursor to the next page
		pageCursor = selected[i-1]
	}
}

// Update is a method for updating user entity.
func (users *users) Update(ctx context.Context, userID uuid.UUID, updateRequest console.UpdateUserRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields, err := users.toUpdateUser(updateRequest)
	if err != nil {
		return err
	}

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(userID[:]),
		*updateFields,
	)

	return err
}

// UpdatePaidTier sets whether the user is in the paid tier.
func (users *users) UpdatePaidTier(ctx context.Context, id uuid.UUID, paidTier bool, projectBandwidthLimit, projectStorageLimit memory.Size, projectSegmentLimit int64, projectLimit int, upgradeTime *time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	userType := console.FreeUser
	if paidTier {
		userType = console.PaidUser
	}
	updateFields := dbx.User_Update_Fields{
		Kind:                  dbx.User_Kind(int(userType)),
		ProjectLimit:          dbx.User_ProjectLimit(projectLimit),
		ProjectBandwidthLimit: dbx.User_ProjectBandwidthLimit(projectBandwidthLimit.Int64()),
		ProjectStorageLimit:   dbx.User_ProjectStorageLimit(projectStorageLimit.Int64()),
		ProjectSegmentLimit:   dbx.User_ProjectSegmentLimit(projectSegmentLimit),
	}
	if paidTier {
		updateFields.TrialExpiration = dbx.User_TrialExpiration_Null()
		updateFields.TrialNotifications = dbx.User_TrialNotifications(0)

		if upgradeTime != nil {
			updateFields.UpgradeTime = dbx.User_UpgradeTime(*upgradeTime)
		}
	}

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(id[:]),
		updateFields,
	)

	return err
}

// UpdateUserAgent is a method to update the user's user agent.
func (users *users) UpdateUserAgent(ctx context.Context, id uuid.UUID, userAgent []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(id[:]),
		dbx.User_Update_Fields{
			UserAgent: dbx.User_UserAgent(userAgent),
		},
	)

	return err
}

// UpdateUserProjectLimits is a method to update the user's usage limits for new projects.
func (users *users) UpdateUserProjectLimits(ctx context.Context, id uuid.UUID, limits console.UsageLimits) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(id[:]),
		dbx.User_Update_Fields{
			ProjectBandwidthLimit: dbx.User_ProjectBandwidthLimit(limits.Bandwidth),
			ProjectStorageLimit:   dbx.User_ProjectStorageLimit(limits.Storage),
			ProjectSegmentLimit:   dbx.User_ProjectSegmentLimit(limits.Segment),
		},
	)

	return err
}

// UpdateDefaultPlacement is a method to update the user's default placement for new projects.
func (users *users) UpdateDefaultPlacement(ctx context.Context, id uuid.UUID, placement storj.PlacementConstraint) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = users.db.Update_User_By_Id(
		ctx,
		dbx.User_Id(id[:]),
		dbx.User_Update_Fields{
			DefaultPlacement: dbx.User_DefaultPlacement(int(placement)),
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

// GetUserKind returns the kind of user.
func (users *users) GetUserKind(ctx context.Context, id uuid.UUID) (kind console.UserKind, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := users.db.Get_User_Kind_By_Id(ctx, dbx.User_Id(id[:]))
	if err != nil {
		return 0, err
	}
	return console.UserKind(row.Kind), nil
}

// GetUpgradeTime is a method for returning a user's upgrade time.
func (users *users) GetUpgradeTime(ctx context.Context, id uuid.UUID) (*time.Time, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	row, err := users.db.Get_User_UpgradeTime_By_Id(ctx, dbx.User_Id(id[:]))
	if err != nil {
		return nil, err
	}
	return row.UpgradeTime, nil
}

// GetSettings is a method for returning a user's set of configurations.
func (users *users) GetSettings(ctx context.Context, userID uuid.UUID) (settings *console.UserSettings, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := users.db.Get_UserSettings_By_UserId(ctx, dbx.UserSettings_UserId(userID[:]))
	if err != nil {
		return nil, err
	}

	settings = &console.UserSettings{}
	settings.OnboardingStart = row.OnboardingStart
	settings.OnboardingEnd = row.OnboardingEnd
	settings.OnboardingStep = row.OnboardingStep
	settings.PassphrasePrompt = true
	if row.PassphrasePrompt != nil {
		settings.PassphrasePrompt = *row.PassphrasePrompt
	}
	if row.SessionMinutes != nil {
		dur := time.Duration(*row.SessionMinutes) * time.Minute
		settings.SessionDuration = &dur
	}

	err = json.Unmarshal(row.NoticeDismissal, &settings.NoticeDismissal)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// UpsertSettings is a method for updating a user's set of configurations if it exists and inserting it otherwise.
func (users *users) UpsertSettings(ctx context.Context, userID uuid.UUID, settings console.UpsertUserSettingsRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	dbID := dbx.UserSettings_UserId(userID[:])
	update := dbx.UserSettings_Update_Fields{}
	fieldCount := 0

	if settings.SessionDuration != nil {
		if *settings.SessionDuration == nil {
			update.SessionMinutes = dbx.UserSettings_SessionMinutes_Null()
		} else {
			update.SessionMinutes = dbx.UserSettings_SessionMinutes(uint((*settings.SessionDuration).Minutes()))
		}
		fieldCount++
	}
	if settings.OnboardingStart != nil {
		update.OnboardingStart = dbx.UserSettings_OnboardingStart(*settings.OnboardingStart)
		fieldCount++
	}
	if settings.OnboardingEnd != nil {
		update.OnboardingEnd = dbx.UserSettings_OnboardingEnd(*settings.OnboardingEnd)
		fieldCount++
	}
	if settings.PassphrasePrompt != nil {
		update.PassphrasePrompt = dbx.UserSettings_PassphrasePrompt(*settings.PassphrasePrompt)
		fieldCount++
	}
	if settings.OnboardingStep != nil {
		update.OnboardingStep = dbx.UserSettings_OnboardingStep(*settings.OnboardingStep)
		fieldCount++
	}

	if settings.NoticeDismissal != nil {
		noticesBytes, err := json.Marshal(settings.NoticeDismissal)
		if err != nil {
			return err
		}
		update.NoticeDismissal = dbx.UserSettings_NoticeDismissal(noticesBytes)
		fieldCount++
	}

	// We need to check whether we are creating a new user, to set default values for onboarding.
	_, err = users.db.Get_UserSettings_By_UserId(ctx, dbID)
	if errors.Is(err, sql.ErrNoRows) {
		create := update
		if settings.OnboardingStart == nil {
			// temporarily inserting as false for new users until we make default for this column false.
			create.OnboardingStart = dbx.UserSettings_OnboardingStart(false)
		}
		if settings.OnboardingEnd == nil {
			// temporarily inserting as false for new users until we make default for this column false.
			create.OnboardingEnd = dbx.UserSettings_OnboardingEnd(false)
		}

		err = users.db.CreateNoReturn_UserSettings(ctx, dbID, dbx.UserSettings_Create_Fields(create))
		if err == nil { // TODO: this should check "already exists", but this should be good enough
			return nil
		}
		err = nil // ignore the error and retry with a regular update
	}
	if err != nil {
		return err
	}

	if fieldCount <= 0 {
		return nil
	}

	_, err = users.db.Update_UserSettings_By_UserId(ctx, dbID, update)
	return err
}

// GetCustomerID returns the customer ID for a given user ID.
func (users *users) GetCustomerID(ctx context.Context, id uuid.UUID) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)
	idRow, err := users.db.Get_StripeCustomer_CustomerId_By_UserId(ctx, dbx.StripeCustomer_UserId(id[:]))
	if err != nil {
		return "", err
	}

	return idRow.CustomerId, nil
}

// SetStatusPendingDeletion set the user to "pending deletion" status safely. It is implemented as
// documented in the corresponding Users interface method that implements.
func (users *users) SetStatusPendingDeletion(
	ctx context.Context, userID uuid.UUID, defaultDaysTillEscalation uint,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	var result sql.Result
	switch users.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		result, err = users.db.ExecContext(ctx, `
					UPDATE users
					SET status = $1,
							status_updated_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
					WHERE id = (
						SELECT u.id
						FROM users AS u JOIN account_freeze_events AS e
							ON e.user_id = u.id
						WHERE u.id = $2
							AND u.status = $3
							AND u.kind = $4
							AND e.event = $5
							AND e.created_at + (COALESCE(e.days_till_escalation, $6) || 'days')::interval < NOW()
							AND 0 = (
								SELECT COUNT(1)
								FROM project_members AS m
								WHERE m.member_id = u.id
									AND m.project_id NOT IN (
										SELECT id FROM projects WHERE owner_id = u.id
									)
							)
					)
			`, console.PendingDeletion, userID.Bytes(), console.Active, console.FreeUser, console.TrialExpirationFreeze,
			defaultDaysTillEscalation,
		)
	case dbutil.Spanner:
		result, err = users.db.ExecContext(ctx, `
					UPDATE users
					SET status = ?,
							status_updated_at = CURRENT_TIMESTAMP
					WHERE id = (
						SELECT u.id
						FROM users AS u JOIN account_freeze_events AS e
							ON u.id = e.user_id
						WHERE u.id = ?
							AND u.status = ?
							AND u.kind = ?
							AND e.event = ?
							AND TIMESTAMP_ADD(e.created_at, INTERVAL COALESCE(e.days_till_escalation, ?) DAY) < CURRENT_TIMESTAMP
							AND 0 = (
								SELECT COUNT(1)
								FROM project_members AS m
								WHERE m.member_id = u.id
									AND m.project_id NOT IN (
										SELECT id FROM projects WHERE owner_id = u.id
									)
							)
					)
			`, console.PendingDeletion, userID.Bytes(), console.Active, console.FreeUser, console.TrialExpirationFreeze,
			defaultDaysTillEscalation,
		)
	default:
		return errs.New("unsupported database dialect: %s", users.impl)
	}

	if err != nil {
		return Error.Wrap(err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return Error.Wrap(err)
	}

	if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ListPendingDeletionBefore returns a list of user IDs that are pending deletion and were marked before the specified time.
// This does not include users that have been frozen.
// NB: This is intended to be used to delete the users this list returns so that every next call
// does not return the same users again.
func (users *users) ListPendingDeletionBefore(
	ctx context.Context,
	limit int,
	before time.Time,
) (page console.UserIDsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	query := users.db.Rebind(`
			SELECT u.id
			FROM users as u
			WHERE u.status = ?
				AND u.status_updated_at < ?
				-- exclude frozen users
				AND (SELECT COUNT(1) FROM account_freeze_events as afe WHERE u.id = afe.user_id) = 0
			ORDER BY u.status_updated_at ASC
			LIMIT ?
		`)

	rows, err := users.db.QueryContext(ctx, query, console.PendingDeletion, before.UTC(), limit+1)
	if err != nil {
		return console.UserIDsPage{}, err
	}
	defer func() { err = errs.Combine(err, rows.Err(), rows.Close()) }()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return console.UserIDsPage{}, errs.Wrap(err)
		}
		ids = append(ids, id)
	}

	if len(ids) == limit+1 {
		page.HasNext = true

		ids = ids[:len(ids)-1]
	}
	page.IDs = ids

	return page, nil
}

// TestSetNow is a method to set the now function for testing purposes.
func (users *users) TestSetNow(nowFn func() time.Time) {
	users.nowFn = nowFn
}

// GetNowFn returns the current time function.
func (users *users) GetNowFn() func() time.Time {
	return users.nowFn
}

// TestingGetAll returns all users.
func (users *users) TestingGetAll(ctx context.Context) (rs []*console.User, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := users.db.All_User(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	for _, row := range rows {
		user, err := UserFromDBX(ctx, row)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		rs = append(rs, user)
	}

	return rs, nil
}

// toUpdateUser creates dbx.User_Update_Fields with only non-empty fields as updatable.
func (users *users) toUpdateUser(request console.UpdateUserRequest) (*dbx.User_Update_Fields, error) {
	update := dbx.User_Update_Fields{}
	if request.FullName != nil {
		update.FullName = dbx.User_FullName(*request.FullName)
	}
	if request.ShortName != nil {
		if *request.ShortName == nil {
			update.ShortName = dbx.User_ShortName_Null()
		} else {
			update.ShortName = dbx.User_ShortName(**request.ShortName)
		}
	}
	if request.Email != nil {
		update.Email = dbx.User_Email(*request.Email)
		update.NormalizedEmail = dbx.User_NormalizedEmail(normalizeEmail(*request.Email))
	}
	if request.PasswordHash != nil {
		if len(request.PasswordHash) > 0 {
			update.PasswordHash = dbx.User_PasswordHash(request.PasswordHash)
		}
	}
	if request.Status != nil {
		update.Status = dbx.User_Status(int(*request.Status))
		update.StatusUpdatedAt = dbx.User_StatusUpdatedAt(users.nowFn())
	}
	if request.UserAgent != nil {
		update.UserAgent = dbx.User_UserAgent(request.UserAgent)
	}
	if request.SignupPromoCode != nil {
		update.SignupPromoCode = dbx.User_SignupPromoCode(*request.SignupPromoCode)
	}
	if request.FinalInvoiceGenerated != nil {
		update.FinalInvoiceGenerated = dbx.User_FinalInvoiceGenerated(*request.FinalInvoiceGenerated)
	}
	if request.ProjectLimit != nil {
		update.ProjectLimit = dbx.User_ProjectLimit(*request.ProjectLimit)
	}
	if request.ProjectStorageLimit != nil {
		update.ProjectStorageLimit = dbx.User_ProjectStorageLimit(*request.ProjectStorageLimit)
	}
	if request.ProjectBandwidthLimit != nil {
		update.ProjectBandwidthLimit = dbx.User_ProjectBandwidthLimit(*request.ProjectBandwidthLimit)
	}
	if request.ProjectSegmentLimit != nil {
		update.ProjectSegmentLimit = dbx.User_ProjectSegmentLimit(*request.ProjectSegmentLimit)
	}
	if request.Kind != nil {
		update.Kind = dbx.User_Kind(int(*request.Kind))
	}
	if request.MFAEnabled != nil {
		update.MfaEnabled = dbx.User_MfaEnabled(*request.MFAEnabled)
	}
	if request.MFASecretKey != nil {
		if *request.MFASecretKey == nil {
			update.MfaSecretKey = dbx.User_MfaSecretKey_Null()
		} else {
			update.MfaSecretKey = dbx.User_MfaSecretKey(**request.MFASecretKey)
		}
	}
	if request.MFARecoveryCodes != nil {
		if *request.MFARecoveryCodes == nil {
			update.MfaRecoveryCodes = dbx.User_MfaRecoveryCodes_Null()
		} else {
			recoveryBytes, err := json.Marshal(*request.MFARecoveryCodes)
			if err != nil {
				return nil, err
			}
			update.MfaRecoveryCodes = dbx.User_MfaRecoveryCodes(string(recoveryBytes))
		}
	}
	if request.FailedLoginCount != nil {
		update.FailedLoginCount = dbx.User_FailedLoginCount(*request.FailedLoginCount)
	}
	if request.LoginLockoutExpiration != nil {
		if *request.LoginLockoutExpiration == nil {
			update.LoginLockoutExpiration = dbx.User_LoginLockoutExpiration_Null()
		} else {
			update.LoginLockoutExpiration = dbx.User_LoginLockoutExpiration(**request.LoginLockoutExpiration)
		}
	}

	if request.DefaultPlacement > 0 {
		update.DefaultPlacement = dbx.User_DefaultPlacement(int(request.DefaultPlacement))
	}

	if request.ActivationCode != nil {
		update.ActivationCode = dbx.User_ActivationCode(*request.ActivationCode)
	}

	if request.SignupId != nil {
		update.SignupId = dbx.User_SignupId(*request.SignupId)
	}

	if request.IsProfessional != nil {
		update.IsProfessional = dbx.User_IsProfessional(*request.IsProfessional)
	}
	if request.HaveSalesContact != nil {
		update.HaveSalesContact = dbx.User_HaveSalesContact(*request.HaveSalesContact)
	}
	if request.Position != nil {
		update.Position = dbx.User_Position(*request.Position)
	}
	if request.CompanyName != nil {
		update.CompanyName = dbx.User_CompanyName(*request.CompanyName)
	}
	if request.EmployeeCount != nil {
		update.EmployeeCount = dbx.User_EmployeeCount(*request.EmployeeCount)
	}

	if request.TrialExpiration != nil {
		update.TrialExpiration = dbx.User_TrialExpiration_Raw(*request.TrialExpiration)
	}
	if request.TrialNotifications != nil {
		update.TrialNotifications = dbx.User_TrialNotifications(int(*request.TrialNotifications))
	}
	if request.UpgradeTime != nil {
		update.UpgradeTime = dbx.User_UpgradeTime(*request.UpgradeTime)
	}

	if request.NewUnverifiedEmail != nil {
		if *request.NewUnverifiedEmail == nil {
			update.NewUnverifiedEmail = dbx.User_NewUnverifiedEmail_Null()
		} else {
			update.NewUnverifiedEmail = dbx.User_NewUnverifiedEmail(**request.NewUnverifiedEmail)
		}
	}
	if request.EmailChangeVerificationStep != nil {
		update.EmailChangeVerificationStep = dbx.User_EmailChangeVerificationStep(*request.EmailChangeVerificationStep)
	}
	if request.ExternalID != nil {
		if *request.ExternalID == nil {
			update.ExternalId = dbx.User_ExternalId_Null()
		} else {
			update.ExternalId = dbx.User_ExternalId(**request.ExternalID)
		}
	}
	if request.TenantID != nil {
		if *request.TenantID == nil || **request.TenantID == "" {
			update.TenantId = dbx.User_TenantId_Null()
		} else {
			update.TenantId = dbx.User_TenantId(**request.TenantID)
		}
	}
	if request.HubspotObjectID != nil {
		if *request.HubspotObjectID == nil {
			update.HubspotObjectId = dbx.User_HubspotObjectId_Null()
		} else {
			update.HubspotObjectId = dbx.User_HubspotObjectId(**request.HubspotObjectID)
		}
	}

	return &update, nil
}

// UserFromDBX is used for creating User entity from autogenerated dbx.User struct.
func UserFromDBX(ctx context.Context, user *dbx.User) (_ *console.User, err error) {
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
		ID:                          id,
		ExternalID:                  user.ExternalId,
		TenantID:                    user.TenantId,
		FullName:                    user.FullName,
		Email:                       user.Email,
		PasswordHash:                user.PasswordHash,
		Status:                      console.UserStatus(user.Status),
		StatusUpdatedAt:             user.StatusUpdatedAt,
		CreatedAt:                   user.CreatedAt,
		ProjectLimit:                user.ProjectLimit,
		ProjectBandwidthLimit:       user.ProjectBandwidthLimit,
		ProjectStorageLimit:         user.ProjectStorageLimit,
		ProjectSegmentLimit:         user.ProjectSegmentLimit,
		Kind:                        console.UserKind(user.Kind),
		IsProfessional:              user.IsProfessional,
		HaveSalesContact:            user.HaveSalesContact,
		MFAEnabled:                  user.MfaEnabled,
		VerificationReminders:       user.VerificationReminders,
		TrialNotifications:          user.TrialNotifications,
		SignupCaptcha:               user.SignupCaptcha,
		TrialExpiration:             user.TrialExpiration,
		UpgradeTime:                 user.UpgradeTime,
		NewUnverifiedEmail:          user.NewUnverifiedEmail,
		EmailChangeVerificationStep: user.EmailChangeVerificationStep,
		FinalInvoiceGenerated:       user.FinalInvoiceGenerated,
		HubspotObjectID:             user.HubspotObjectId,
	}

	if user.DefaultPlacement != nil {
		result.DefaultPlacement = storj.PlacementConstraint(*user.DefaultPlacement)
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

	if user.FailedLoginCount != nil {
		result.FailedLoginCount = *user.FailedLoginCount
	}

	if user.LoginLockoutExpiration != nil {
		result.LoginLockoutExpiration = *user.LoginLockoutExpiration
	}

	if user.ActivationCode != nil {
		result.ActivationCode = *user.ActivationCode
	}

	if user.SignupId != nil {
		result.SignupId = *user.SignupId
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

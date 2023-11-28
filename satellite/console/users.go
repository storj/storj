// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"net/mail"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/consoleauth"
)

// Users exposes methods to manage User table in database.
//
// architecture: Database
type Users interface {
	// Get is a method for querying user from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*User, error)
	// GetUnverifiedNeedingReminder gets unverified users needing a reminder to verify their email.
	GetUnverifiedNeedingReminder(ctx context.Context, firstReminder, secondReminder, cutoff time.Time) ([]*User, error)
	// UpdateVerificationReminders increments verification_reminders.
	UpdateVerificationReminders(ctx context.Context, id uuid.UUID) error
	// UpdateFailedLoginCountAndExpiration increments failed_login_count and sets login_lockout_expiration appropriately.
	UpdateFailedLoginCountAndExpiration(ctx context.Context, failedLoginPenalty *float64, id uuid.UUID) error
	// GetByEmailWithUnverified is a method for querying users by email from the database.
	GetByEmailWithUnverified(ctx context.Context, email string) (verified *User, unverified []User, err error)
	// GetByStatus is a method for querying user by status from the database.
	GetByStatus(ctx context.Context, status UserStatus, cursor UserCursor) (*UsersPage, error)
	// GetByEmail is a method for querying user by verified email from the database.
	GetByEmail(ctx context.Context, email string) (*User, error)
	// Insert is a method for inserting user into the database.
	Insert(ctx context.Context, user *User) (*User, error)
	// Delete is a method for deleting user by ID from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteUnverifiedBefore deletes unverified users created prior to some time from the database.
	DeleteUnverifiedBefore(ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration, pageSize int) error
	// Update is a method for updating user entity.
	Update(ctx context.Context, userID uuid.UUID, request UpdateUserRequest) error
	// UpdatePaidTier sets whether the user is in the paid tier.
	UpdatePaidTier(ctx context.Context, id uuid.UUID, paidTier bool, projectBandwidthLimit, projectStorageLimit memory.Size, projectSegmentLimit int64, projectLimit int) error
	// UpdateUserAgent is a method to update the user's user agent.
	UpdateUserAgent(ctx context.Context, id uuid.UUID, userAgent []byte) error
	// UpdateUserProjectLimits is a method to update the user's usage limits for new projects.
	UpdateUserProjectLimits(ctx context.Context, id uuid.UUID, limits UsageLimits) error
	// UpdateDefaultPlacement is a method to update the user's default placement for new projects.
	UpdateDefaultPlacement(ctx context.Context, id uuid.UUID, placement storj.PlacementConstraint) error
	// GetProjectLimit is a method to get the users project limit
	GetProjectLimit(ctx context.Context, id uuid.UUID) (limit int, err error)
	// GetUserProjectLimits is a method to get the users storage and bandwidth limits for new projects.
	GetUserProjectLimits(ctx context.Context, id uuid.UUID) (limit *ProjectLimits, err error)
	// GetUserPaidTier is a method to gather whether the specified user is on the Paid Tier or not.
	GetUserPaidTier(ctx context.Context, id uuid.UUID) (isPaid bool, err error)
	// GetSettings is a method for returning a user's set of configurations.
	GetSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error)
	// UpsertSettings is a method for updating a user's set of configurations if it exists and inserting it otherwise.
	UpsertSettings(ctx context.Context, userID uuid.UUID, settings UpsertUserSettingsRequest) error
}

// UserInfo holds User updatable data.
type UserInfo struct {
	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`
}

// UserCursor holds info for user info cursor pagination.
type UserCursor struct {
	Limit uint `json:"limit"`
	Page  uint `json:"page"`
}

// UsersPage represent user info page result.
type UsersPage struct {
	Users []User `json:"users"`

	Limit  uint   `json:"limit"`
	Offset uint64 `json:"offset"`

	PageCount   uint   `json:"pageCount"`
	CurrentPage uint   `json:"currentPage"`
	TotalCount  uint64 `json:"totalCount"`
}

// IsValid checks UserInfo validity and returns error describing whats wrong.
// The returned error has the class ErrValiation.
func (user *UserInfo) IsValid() error {
	// validate fullName
	if err := ValidateFullName(user.FullName); err != nil {
		return ErrValidation.Wrap(err)
	}

	return nil
}

// CreateUser struct holds info for User creation.
type CreateUser struct {
	FullName         string `json:"fullName"`
	ShortName        string `json:"shortName"`
	Email            string `json:"email"`
	UserAgent        []byte `json:"userAgent"`
	Password         string `json:"password"`
	IsProfessional   bool   `json:"isProfessional"`
	Position         string `json:"position"`
	CompanyName      string `json:"companyName"`
	WorkingOn        string `json:"workingOn"`
	EmployeeCount    string `json:"employeeCount"`
	HaveSalesContact bool   `json:"haveSalesContact"`
	CaptchaResponse  string `json:"captchaResponse"`
	IP               string `json:"ip"`
	SignupPromoCode  string `json:"signupPromoCode"`
	ActivationCode   string `json:"-"`
	SignupId         string `json:"-"`
}

// IsValid checks CreateUser validity and returns error describing whats wrong.
// The returned error has the class ErrValiation.
func (user *CreateUser) IsValid() error {
	errgrp := errs.Group{}

	errgrp.Add(
		ValidateFullName(user.FullName),
		ValidateNewPassword(user.Password),
	)

	// validate email
	_, err := mail.ParseAddress(user.Email)
	errgrp.Add(err)

	return ErrValidation.Wrap(errgrp.Err())
}

// ProjectLimits holds info for a users bandwidth and storage limits for new projects.
type ProjectLimits struct {
	ProjectBandwidthLimit memory.Size `json:"projectBandwidthLimit"`
	ProjectStorageLimit   memory.Size `json:"projectStorageLimit"`
	ProjectSegmentLimit   int64       `json:"projectSegmentLimit"`
}

// AuthUser holds info for user authentication token requests.
type AuthUser struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	MFAPasscode     string `json:"mfaPasscode"`
	MFARecoveryCode string `json:"mfaRecoveryCode"`
	CaptchaResponse string `json:"captchaResponse"`
	IP              string `json:"-"`
	UserAgent       string `json:"-"`
}

// TokenInfo holds info for user authentication token responses.
type TokenInfo struct {
	consoleauth.Token `json:"token"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

// UserStatus - is used to indicate status of the users account.
type UserStatus int

const (
	// Inactive is a user status that he receives after registration.
	Inactive UserStatus = 0
	// Active is a user status that he receives after account activation.
	Active UserStatus = 1
	// Deleted is a user status that he receives after deleting account.
	Deleted UserStatus = 2
	// PendingDeletion is a user status that he receives before deleting account.
	PendingDeletion UserStatus = 3
	// LegalHold is a user status that he receives for legal reasons.
	LegalHold UserStatus = 4
)

// String returns a string representation of the user status.
func (s UserStatus) String() string {
	switch s {
	case Inactive:
		return "Inactive"
	case Active:
		return "Active"
	case Deleted:
		return "Deleted"
	case PendingDeletion:
		return "Pending Deletion"
	case LegalHold:
		return "Legal Hold"
	default:
		return ""
	}
}

// User is a database object that describes User entity.
type User struct {
	ID uuid.UUID `json:"id"`

	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`

	Email        string `json:"email"`
	PasswordHash []byte `json:"passwordHash"`

	Status    UserStatus `json:"status"`
	UserAgent []byte     `json:"userAgent"`

	CreatedAt time.Time `json:"createdAt"`

	ProjectLimit          int   `json:"projectLimit"`
	ProjectStorageLimit   int64 `json:"projectStorageLimit"`
	ProjectBandwidthLimit int64 `json:"projectBandwidthLimit"`
	ProjectSegmentLimit   int64 `json:"projectSegmentLimit"`
	PaidTier              bool  `json:"paidTier"`

	IsProfessional bool   `json:"isProfessional"`
	Position       string `json:"position"`
	CompanyName    string `json:"companyName"`
	CompanySize    int    `json:"companySize"`
	WorkingOn      string `json:"workingOn"`
	EmployeeCount  string `json:"employeeCount"`

	HaveSalesContact bool `json:"haveSalesContact"`

	MFAEnabled       bool     `json:"mfaEnabled"`
	MFASecretKey     string   `json:"mfaSecretKey"`
	MFARecoveryCodes []string `json:"mfaRecoveryCodes"`

	SignupPromoCode string `json:"signupPromoCode"`

	VerificationReminders int `json:"verificationReminders"`

	FailedLoginCount       int       `json:"failedLoginCount"`
	LoginLockoutExpiration time.Time `json:"loginLockoutExpiration"`
	SignupCaptcha          *float64  `json:"-"`

	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`

	ActivationCode string `json:"-"`
	SignupId       string `json:"-"`
}

// ResponseUser is an entity which describes db User and can be sent in response.
type ResponseUser struct {
	ID                   uuid.UUID `json:"id"`
	FullName             string    `json:"fullName"`
	ShortName            string    `json:"shortName"`
	Email                string    `json:"email"`
	UserAgent            []byte    `json:"userAgent"`
	ProjectLimit         int       `json:"projectLimit"`
	IsProfessional       bool      `json:"isProfessional"`
	Position             string    `json:"position"`
	CompanyName          string    `json:"companyName"`
	EmployeeCount        string    `json:"employeeCount"`
	HaveSalesContact     bool      `json:"haveSalesContact"`
	PaidTier             bool      `json:"paidTier"`
	MFAEnabled           bool      `json:"isMFAEnabled"`
	MFARecoveryCodeCount int       `json:"mfaRecoveryCodeCount"`
}

// key is a context value key type.
type key int

// userKey is context key for User.
const userKey key = 0

// WithUser creates new context with User.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetUser gets User from context.
func GetUser(ctx context.Context) (*User, error) {
	if user, ok := ctx.Value(userKey).(*User); ok {
		return user, nil
	}

	return nil, Error.New("user is not in context")
}

// UpdateUserRequest contains all columns which are optionally updatable by users.Update.
type UpdateUserRequest struct {
	FullName  *string
	ShortName **string

	Email        *string
	PasswordHash []byte

	Status *UserStatus

	ProjectLimit          *int
	ProjectStorageLimit   *int64
	ProjectBandwidthLimit *int64
	ProjectSegmentLimit   *int64
	PaidTier              *bool

	MFAEnabled       *bool
	MFASecretKey     **string
	MFARecoveryCodes *[]string

	// failed_login_count is nullable, but we don't really have a reason
	// to set it to NULL, so it doesn't need to be a double pointer here.
	FailedLoginCount *int

	LoginLockoutExpiration **time.Time

	DefaultPlacement storj.PlacementConstraint

	ActivationCode *string
	SignupId       *string
}

// UserSettings contains configurations for a user.
type UserSettings struct {
	SessionDuration  *time.Duration `json:"sessionDuration"`
	OnboardingStart  bool           `json:"onboardingStart"`
	OnboardingEnd    bool           `json:"onboardingEnd"`
	PassphrasePrompt bool           `json:"passphrasePrompt"`
	OnboardingStep   *string        `json:"onboardingStep"`
}

// UpsertUserSettingsRequest contains all user settings which are configurable via Users.UpsertSettings.
type UpsertUserSettingsRequest struct {
	// The DB stores this value with minute granularity. Finer time units are ignored.
	SessionDuration  **time.Duration
	OnboardingStart  *bool
	OnboardingEnd    *bool
	PassphrasePrompt *bool
	OnboardingStep   *string
}

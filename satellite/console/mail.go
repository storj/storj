// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package console

import "time"

// TrialExpirationReminderEmail is mailservice template with trial expiration reminder data.
type TrialExpirationReminderEmail struct {
	Origin              string
	SignInLink          string
	ContactInfoURL      string
	ScheduleMeetingLink string
}

// Template returns email template name.
func (*TrialExpirationReminderEmail) Template() string { return "TrialExpirationReminder" }

// Subject gets email subject.
func (*TrialExpirationReminderEmail) Subject() string { return "Your Storj trial is ending soon" }

// TrialExpiredEmail is mailservice template with trial expiration data.
type TrialExpiredEmail struct {
	Origin              string
	SignInLink          string
	ContactInfoURL      string
	ScheduleMeetingLink string
}

// Template returns email template name.
func (*TrialExpiredEmail) Template() string { return "TrialExpired" }

// Subject gets email subject.
func (*TrialExpiredEmail) Subject() string {
	return "Your Storj trial has ended - Act now to continue!"
}

// AccountActivationEmail is mailservice template with activation data.
type AccountActivationEmail struct {
	Origin                string
	ActivationLink        string
	ContactInfoURL        string
	TermsAndConditionsURL string
}

// Template returns email template name.
func (*AccountActivationEmail) Template() string { return "Welcome" }

// Subject gets email subject.
func (*AccountActivationEmail) Subject() string { return "Activate your email" }

// AccountActivationCodeEmail is mailservice template with activation code.
type AccountActivationCodeEmail struct {
	ActivationCode string
}

// Template returns email template name.
func (*AccountActivationCodeEmail) Template() string { return "WelcomeWithCode" }

// Subject gets email subject.
func (*AccountActivationCodeEmail) Subject() string { return "Activate your email" }

// ChangeEmailSuccessEmail is mailservice template to notify user about successful email change.
type ChangeEmailSuccessEmail struct{}

// Template returns email template name.
func (*ChangeEmailSuccessEmail) Template() string { return "EmailChangeSuccess" }

// Subject gets email subject.
func (*ChangeEmailSuccessEmail) Subject() string { return "Email has been changed" }

// RequestAccountDeletionSuccessEmail is mailservice template to notify user about successful account delete request.
type RequestAccountDeletionSuccessEmail struct{}

// Template returns email template name.
func (*RequestAccountDeletionSuccessEmail) Template() string { return "RequestAccountDeletionSuccess" }

// Subject gets email subject.
func (*RequestAccountDeletionSuccessEmail) Subject() string { return "Account deletion" }

// EmailAddressVerificationEmail is mailservice template with a verification code.
type EmailAddressVerificationEmail struct {
	Action           string
	VerificationCode string
}

// Template returns email template name.
func (*EmailAddressVerificationEmail) Template() string { return "EmailAddressVerification" }

// Subject gets email subject.
func (*EmailAddressVerificationEmail) Subject() string { return "Verify your email" }

// ForgotPasswordEmail is mailservice template with reset password data.
type ForgotPasswordEmail struct {
	Origin                     string
	ResetLink                  string
	CancelPasswordRecoveryLink string
	LetUsKnowURL               string
	ContactInfoURL             string
	TermsAndConditionsURL      string
}

// Template returns email template name.
func (*ForgotPasswordEmail) Template() string { return "Forgot" }

// Subject gets email subject.
func (*ForgotPasswordEmail) Subject() string { return "Password recovery request" }

// PasswordChangedEmail is mailservice template with password changed data.
type PasswordChangedEmail struct {
	ResetPasswordLink string
}

// Template returns email template name.
func (*PasswordChangedEmail) Template() string { return "PasswordChanged" }

// Subject gets email subject.
func (*PasswordChangedEmail) Subject() string { return "Your password changed" }

// ProjectInvitationEmail is mailservice template for project invitation email.
type ProjectInvitationEmail struct {
	Origin                string
	InviterEmail          string
	SignInLink            string
	LetUsKnowURL          string
	ContactInfoURL        string
	TermsAndConditionsURL string
}

// Template returns email template name.
func (*ProjectInvitationEmail) Template() string { return "Invite" }

// Subject gets email subject.
func (email *ProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// ExistingUserProjectInvitationEmail is mailservice template for project invitation email for existing users.
type ExistingUserProjectInvitationEmail struct {
	InviterEmail string
	Region       string
	SignInLink   string
}

// Template returns email template name.
func (*ExistingUserProjectInvitationEmail) Template() string { return "ExistingUserInvite" }

// Subject gets email subject.
func (email *ExistingUserProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// UnverifiedUserProjectInvitationEmail is mailservice template for project invitation email for unverified users.
type UnverifiedUserProjectInvitationEmail struct {
	InviterEmail   string
	Region         string
	ActivationLink string
}

// Template returns email template name.
func (*UnverifiedUserProjectInvitationEmail) Template() string { return "UnverifiedUserInvite" }

// Subject gets email subject.
func (email *UnverifiedUserProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// NewUserProjectInvitationEmail is mailservice template for project invitation email for new users.
type NewUserProjectInvitationEmail struct {
	InviterEmail string
	Region       string
	SignUpLink   string
}

// Template returns email template name.
func (*NewUserProjectInvitationEmail) Template() string { return "NewUserInvite" }

// Subject gets email subject.
func (email *NewUserProjectInvitationEmail) Subject() string {
	return "You were invited to join a project on Storj"
}

// UnknownResetPasswordEmail is mailservice template with unknown password reset data.
type UnknownResetPasswordEmail struct {
	Satellite           string
	Email               string
	DoubleCheckLink     string
	ResetPasswordLink   string
	CreateAnAccountLink string
	SupportTeamLink     string
}

// Template returns email template name.
func (*UnknownResetPasswordEmail) Template() string { return "UnknownReset" }

// Subject gets email subject.
func (*UnknownResetPasswordEmail) Subject() string {
	return "You have requested to reset your password, but..."
}

// AccountAlreadyExistsEmail is mailservice template for email where user tries to create account, but one already exists.
type AccountAlreadyExistsEmail struct {
	Origin            string
	SatelliteName     string
	SignInLink        string
	ResetPasswordLink string
	CreateAccountLink string
}

// Template returns email template name.
func (*AccountAlreadyExistsEmail) Template() string { return "AccountAlreadyExists" }

// Subject gets email subject.
func (*AccountAlreadyExistsEmail) Subject() string {
	return "Are you trying to sign in?"
}

// LockAccountActivityType is an auth activity type which led to account lock.
type LockAccountActivityType = string

const (
	// LoginAccountLock represents an account lock activity type triggered by multiple failed login attempts.
	LoginAccountLock LockAccountActivityType = "login"

	// MfaAccountLock stands for "2fa check" and represents an account lock activity type triggered by multiple failed two-factor authentication attempts.
	MfaAccountLock LockAccountActivityType = "2fa check"

	// ChangeEmailLock stands for "change email" and represents an account lock activity type triggered by multiple failed change email actions.
	ChangeEmailLock LockAccountActivityType = "change email"
)

// LoginLockAccountEmail is mailservice template with login lock account data.
type LoginLockAccountEmail struct {
	LockoutDuration   time.Duration
	ResetPasswordLink string
	ActivityType      LockAccountActivityType
}

// Template returns email template name.
func (*LoginLockAccountEmail) Template() string { return "LoginLockAccount" }

// Subject gets email subject.
func (*LoginLockAccountEmail) Subject() string { return "Account Lock" }

// ActivationLockAccountEmail is mailservice template with activation lock account data.
type ActivationLockAccountEmail struct {
	LockoutDuration time.Duration
	SupportURL      string
}

// Template returns email template name.
func (*ActivationLockAccountEmail) Template() string { return "ActivationLockAccount" }

// Subject gets email subject.
func (*ActivationLockAccountEmail) Subject() string { return "Account Lock" }

// BillingWarningEmail is an email sent to notify users of billing warning event.
type BillingWarningEmail struct {
	EmailNumber int
	Days        int
	SignInLink  string
	SupportLink string
}

// Template returns email template name.
func (*BillingWarningEmail) Template() string { return "BillingWarning" }

// Subject gets email subject.
func (*BillingWarningEmail) Subject() string {
	return "Your payment is outstanding - Act now to continue!"
}

// BillingFreezeNotificationEmail is an email sent to notify users of account freeze event.
type BillingFreezeNotificationEmail struct {
	EmailNumber int
	Days        int
	SignInLink  string
	SupportLink string
}

// Template returns email template name.
func (*BillingFreezeNotificationEmail) Template() string { return "BillingFreezeNotification" }

// Subject gets email subject.
func (b *BillingFreezeNotificationEmail) Subject() string {
	title := "Your account has been suspended"
	if b.Days <= 0 {
		title = "Your data is marked for deletion"
	}
	return title + " - Act now to continue!"
}

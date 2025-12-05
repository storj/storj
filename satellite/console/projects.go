// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// Projects exposes methods to manage Project table in database.
//
// architecture: Database
type Projects interface {
	// GetAll is a method for querying all projects from the database.
	GetAll(ctx context.Context) ([]Project, error)
	// GetCreatedBefore retrieves all projects created before provided date.
	GetCreatedBefore(ctx context.Context, before time.Time) ([]Project, error)
	// GetByUserID returns a list of projects (including disabled) where user is a project member.
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// GetActiveByUserID returns a list of active projects where user is a project member.
	GetActiveByUserID(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// GetOwn returns a list of projects (including disabled) where user is an owner.
	GetOwn(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// GetOwnActive returns a list of active projects where user is an owner.
	GetOwnActive(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// Get is a method for querying project from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*Project, error)
	// GetSalt returns the project's salt.
	GetSalt(ctx context.Context, id uuid.UUID) ([]byte, error)
	// GetEncryptedPassphrase gets the encrypted passphrase of this project.
	// NB: projects that don't have satellite managed encryption will not have this.
	GetEncryptedPassphrase(ctx context.Context, id uuid.UUID) ([]byte, *int, error)
	// GetByPublicID is a method for querying project from the database by public_id.
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (*Project, error)
	// GetByPublicOrPrivateID is a method for querying project from the database by either publicID or id.
	GetByPublicOrPrivateID(ctx context.Context, id uuid.UUID) (*Project, error)
	// GetPublicID returns the public project ID for a given project ID.
	GetPublicID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// Insert is a method for inserting project into the database.
	Insert(ctx context.Context, project *Project) (*Project, error)
	// Delete is a method for deleting project by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating project entity.
	Update(ctx context.Context, project *Project) error
	// List returns paginated projects, created before provided timestamp.
	List(ctx context.Context, offset int64, limit int, before time.Time) (ProjectsPage, error)
	// ListByOwnerID is a method for querying all projects (including disabled) from the database by ownerID. It also includes the number of members for each project.
	ListByOwnerID(ctx context.Context, userID uuid.UUID, cursor ProjectsCursor) (ProjectsPage, error)
	// ListActiveByOwnerID is a method for querying only active projects from the database by ownerID. It also includes the number of members for each project.
	ListActiveByOwnerID(ctx context.Context, userID uuid.UUID, cursor ProjectsCursor) (ProjectsPage, error)

	// UpdateRateLimit is a method for updating projects rate limit.
	UpdateRateLimit(ctx context.Context, id uuid.UUID, newLimit *int) error

	// UpdateBurstLimit is a method for updating projects burst limit.
	UpdateBurstLimit(ctx context.Context, id uuid.UUID, newLimit *int) error

	// GetMaxBuckets is a method to get the maximum number of buckets allowed for the project
	GetMaxBuckets(ctx context.Context, id uuid.UUID) (*int, error)
	// GetDefaultVersioning is a method to get the default versioning state of a new bucket in the project.
	GetDefaultVersioning(ctx context.Context, id uuid.UUID) (DefaultVersioning, error)
	// UpdateDefaultVersioning is a method to update the default versioning state of a new bucket in the project.
	UpdateDefaultVersioning(ctx context.Context, id uuid.UUID, versioning DefaultVersioning) error
	// UpdateBucketLimit is a method for updating projects bucket limit.
	UpdateBucketLimit(ctx context.Context, id uuid.UUID, newLimit *int) error

	// UpdateUsageLimits is a method for updating project's usage limits.
	UpdateUsageLimits(ctx context.Context, id uuid.UUID, limits UsageLimits) error

	// UpdateAllLimits is a method for updating max buckets, storage, bandwidth, segment, rate, and burst limits.
	UpdateAllLimits(ctx context.Context, id uuid.UUID, storage, bandwidth, segment *int64, buckets, rate, burst *int) error

	// UpdateLimitsGeneric is a method for updating any or all types of limits on a project.
	// ALL limits passed in to the request will be updated i.e. if a limit type is passed in with a null value, that limit will be updated to null.
	UpdateLimitsGeneric(ctx context.Context, id uuid.UUID, toUpdate []Limit) error

	// UpdateUserAgent is a method for updating projects user agent.
	UpdateUserAgent(ctx context.Context, id uuid.UUID, userAgent []byte) error

	// UpdateStatus is a method for updating projects status.
	UpdateStatus(ctx context.Context, id uuid.UUID, status ProjectStatus) error

	// UpdateDefaultPlacement is a method to update the project's default placement for new segments.
	UpdateDefaultPlacement(ctx context.Context, id uuid.UUID, placement storj.PlacementConstraint) error

	// ListPendingDeletionBefore returns a list of project and owner IDs that are pending deletion and were marked before the specified time.
	ListPendingDeletionBefore(ctx context.Context, offset int64, limit int, before time.Time) (page ProjectIdOwnerIdPage, err error)

	// GetNowFn returns the current time function.
	GetNowFn() func() time.Time
	// TestSetNowFn is used to set the current time for testing purposes.
	TestSetNowFn(func() time.Time)
}

// UsageLimitsConfig is a configuration struct for default per-project usage limits.
type UsageLimitsConfig struct {
	Storage   StorageLimitConfig
	Bandwidth BandwidthLimitConfig
	Segment   SegmentLimitConfig
	Project   ProjectLimitConfig
}

// StorageLimitConfig is a configuration struct for default storage per-project usage limits.
type StorageLimitConfig struct {
	Free memory.Size `help:"the default free-tier storage usage limit" default:"25.00GB" testDefault:"25.00 GB"`
	Paid memory.Size `help:"the default paid-tier storage usage limit" default:"100.00TB" testDefault:"25.00 GB"`
	Nfr  memory.Size `help:"the default NFR storage usage limit" default:"10.00TB" testDefault:"25.00 GB"`
}

// BandwidthLimitConfig is a configuration struct for default bandwidth per-project usage limits.
type BandwidthLimitConfig struct {
	Free memory.Size `help:"the default free-tier bandwidth usage limit" default:"25.00GB"  testDefault:"25.00 GB"`
	Paid memory.Size `help:"the default paid-tier bandwidth usage limit" default:"150.00TB" testDefault:"25.00 GB"`
	Nfr  memory.Size `help:"the default NFR bandwidth usage limit" default:"15.00TB" testDefault:"25.00 GB"`
}

// SegmentLimitConfig is a configuration struct for default segments per-project usage limits.
type SegmentLimitConfig struct {
	Free int64 `help:"the default free-tier segment usage limit" default:"10000"`
	Paid int64 `help:"the default paid-tier segment usage limit" default:"100000000"`
	Nfr  int64 `help:"the default NFR segment usage limit" default:"10000000"`
}

// ProjectLimitConfig is a configuration struct for default project limits.
type ProjectLimitConfig struct {
	Free int `help:"the default free-tier project limit" default:"1" testDefault:"10"`
	Paid int `help:"the default paid-tier project limit" default:"3"`
	Nfr  int `help:"the default NFR project limit" default:"1"`
}

// Project is a database object that describes Project entity.
type Project struct {
	ID       uuid.UUID `json:"id"`
	PublicID uuid.UUID `json:"publicId"`

	Name            string         `json:"name"`
	Description     string         `json:"description"`
	UserAgent       []byte         `json:"userAgent"`
	OwnerID         uuid.UUID      `json:"ownerId"`
	MaxBuckets      *int           `json:"maxBuckets"`
	CreatedAt       time.Time      `json:"createdAt"`
	MemberCount     int            `json:"memberCount"`
	Status          *ProjectStatus `json:"status"`
	StatusUpdatedAt *time.Time     `json:"-"`

	StorageLimit                *memory.Size `json:"storageLimit"`
	StorageUsed                 int64        `json:"-"`
	BandwidthLimit              *memory.Size `json:"bandwidthLimit"`
	BandwidthUsed               int64        `json:"-"`
	UserSpecifiedStorageLimit   *memory.Size `json:"userSpecifiedStorageLimit"`
	UserSpecifiedBandwidthLimit *memory.Size `json:"userSpecifiedBandwidthLimit"`
	SegmentLimit                *int64       `json:"segmentLimit"`

	RateLimit        *int `json:"rateLimit"`
	BurstLimit       *int `json:"burstLimit"`
	RateLimitHead    *int `json:"rateLimitHead,omitempty"`
	BurstLimitHead   *int `json:"burstLimitHead,omitempty"`
	RateLimitGet     *int `json:"rateLimitGet,omitempty"`
	BurstLimitGet    *int `json:"burstLimitGet,omitempty"`
	RateLimitPut     *int `json:"rateLimitPut,omitempty"`
	BurstLimitPut    *int `json:"burstLimitPut,omitempty"`
	RateLimitList    *int `json:"rateLimitList,omitempty"`
	BurstLimitList   *int `json:"burstLimitList,omitempty"`
	RateLimitDelete  *int `json:"rateLimitDelete,omitempty"`
	BurstLimitDelete *int `json:"burstLimitDelete,omitempty"`

	DefaultPlacement   storj.PlacementConstraint `json:"defaultPlacement"`
	DefaultVersioning  DefaultVersioning         `json:"defaultVersioning"`
	PassphraseEnc      []byte                    `json:"-"`
	PassphraseEncKeyID *int                      `json:"-"`
	PathEncryption     *bool                     `json:"-"`

	IsClassic bool `json:"isClassic"`
}

// ProjectStatus - is used to indicate status of the user's project.
type ProjectStatus int

const (
	// ProjectDisabled is a status that project receives after deleting/disabling by the user.
	ProjectDisabled ProjectStatus = 0
	// ProjectActive is a status that project receives after creation.
	ProjectActive ProjectStatus = 1
	// ProjectPendingDeletion is a status that project receives after user initiates deletion
	// in the abbreviated flow, but before the project is fully deleted.
	ProjectPendingDeletion ProjectStatus = 2
)

// ProjectStatuses are all valid project statuses.
var ProjectStatuses = []ProjectStatus{ProjectDisabled, ProjectActive, ProjectPendingDeletion}

// String returns the string name.
func (status *ProjectStatus) String() string {
	if status == nil {
		return "unset"
	}
	switch *status {
	case ProjectDisabled:
		return "Disabled"
	case ProjectActive:
		return "Active"
	case ProjectPendingDeletion:
		return "Pending Deletion"
	default:
		return fmt.Sprintf("unknown ProjectStatus(%d)", *status)
	}
}

// UpsertProjectInfo holds data needed to create/update Project.
type UpsertProjectInfo struct {
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	StorageLimit   *memory.Size `json:"storageLimit"`
	BandwidthLimit *memory.Size `json:"bandwidthLimit"`

	// these fields are only used for inserts and ignored for updates
	CreatedAt        time.Time `json:"createdAt"`
	ManagePassphrase bool      `json:"managePassphrase"`
}

// UpdateLimitsInfo holds data needed to update project limits.
type UpdateLimitsInfo struct {
	StorageLimit   *memory.Size `json:"storageLimit"`
	BandwidthLimit *memory.Size `json:"bandwidthLimit"`
}

// ProjectInfo holds data sent via user facing http endpoints.
type ProjectInfo struct {
	ID                   uuid.UUID                 `json:"id"`
	Name                 string                    `json:"name"`
	OwnerID              uuid.UUID                 `json:"ownerId"`
	Description          string                    `json:"description"`
	MemberCount          int                       `json:"memberCount"`
	CreatedAt            time.Time                 `json:"createdAt"`
	EdgeURLOverrides     *EdgeURLOverrides         `json:"edgeURLOverrides,omitempty"`
	StorageUsed          int64                     `json:"storageUsed"`
	BandwidthUsed        int64                     `json:"bandwidthUsed"`
	Versioning           DefaultVersioning         `json:"versioning"`
	Placement            storj.PlacementConstraint `json:"placement"`
	HasManagedPassphrase bool                      `json:"hasManagedPassphrase"`
	IsClassic            bool                      `json:"isClassic"`
}

// DefaultVersioning represents the default versioning state of a new bucket in the project.
type DefaultVersioning int

const (
	// VersioningUnsupported - versioning for created buckets is not supported.
	VersioningUnsupported DefaultVersioning = 0
	// Unversioned - versioning for created buckets is supported but not enabled.
	Unversioned DefaultVersioning = 1
	// VersioningEnabled - versioning for created buckets is supported and enabled.
	VersioningEnabled DefaultVersioning = 2
	// Note: suspended is not a valid state for new buckets.
)

// ProjectsCursor holds info for project
// cursor pagination.
type ProjectsCursor struct {
	Limit int
	Page  int
}

// PageInfo contains details about a pagination
// result set.
type PageInfo struct {
	Next       bool
	NextOffset int64

	Limit  int
	Offset int64

	PageCount   int
	CurrentPage int
	TotalCount  int64
}

// ProjectsPage returns paginated projects.
type ProjectsPage struct {
	PageInfo
	Projects []Project
}

// ProjectIdOwnerId holds a project ID and its owner's ID.
type ProjectIdOwnerId struct {
	ProjectID       uuid.UUID
	ProjectPublicID uuid.UUID
	OwnerID         uuid.UUID
}

// ProjectIdOwnerIdPage holds a page of project IDs and their owner IDs.
type ProjectIdOwnerIdPage struct {
	PageInfo
	Ids []ProjectIdOwnerId
}

// LimitRequestInfo holds data needed to request limit increase.
type LimitRequestInfo struct {
	LimitType    string      `json:"limitType"`
	CurrentLimit memory.Size `json:"currentLimit"`
	DesiredLimit memory.Size `json:"desiredLimit"`
}

// ProjectConfig holds config for available "features" for a project.
type ProjectConfig struct {
	// HasManagedPassphrase is a failsafe to prevent user-managed-encryption behavior in the UI if
	// managed encryption is enabled for a project, but the satellite is unable to decrypt the passphrase.
	HasManagedPassphrase bool              `json:"hasManagedPassphrase"`
	EncryptPath          bool              `json:"encryptPath"`
	Passphrase           string            `json:"passphrase,omitempty"`
	IsOwnerPaidTier      bool              `json:"isOwnerPaidTier"`
	HasPaidPrivileges    bool              `json:"hasPaidPrivileges"`
	Role                 ProjectMemberRole `json:"role"`
	Salt                 string            `json:"salt"`
	MembersCount         uint64            `json:"membersCount"`
	AvailablePlacements  []PlacementDetail `json:"availablePlacements"`
	ComputeAuthToken     string            `json:"computeAuthToken,omitempty"`
}

// DeleteProjectInfo holds data for project deletion UI flow.
type DeleteProjectInfo struct {
	LockEnabledBuckets  int             `json:"lockEnabledBuckets"`
	Buckets             int             `json:"buckets"`
	APIKeys             int             `json:"apiKeys"`
	CurrentUsage        bool            `json:"currentUsage"`
	CurrentMonthPrice   decimal.Decimal `json:"-"`
	InvoicingIncomplete bool            `json:"invoicingIncomplete"`
}

// ValidateNameAndDescription validates project name and description strings.
// Project name must have more than 0 and less than 21 symbols.
// Project description can't have more than hundred symbols.
func ValidateNameAndDescription(name string, description string) error {
	if len(name) == 0 {
		return errors.New("project name can't be empty")
	}

	if len(name) > 20 {
		return errors.New("project name can't have more than 20 symbols")
	}

	if len(description) > 100 {
		return errors.New("project description can't have more than 100 symbols")
	}

	return nil
}

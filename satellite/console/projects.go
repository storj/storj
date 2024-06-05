// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"errors"
	"time"

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
	// GetByUserID returns a list of projects where user is a project member.
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// GetOwn returns a list of projects where user is an owner.
	GetOwn(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// Get is a method for querying project from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*Project, error)
	// GetSalt returns the project's salt.
	GetSalt(ctx context.Context, id uuid.UUID) ([]byte, error)
	// GetEncryptedPassphrase gets the encrypted passphrase of this project.
	// NB: projects that don't have satellite managed encryption will not have this.
	GetEncryptedPassphrase(ctx context.Context, id uuid.UUID) ([]byte, error)
	// GetByPublicID is a method for querying project from the database by public_id.
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (*Project, error)
	// Insert is a method for inserting project into the database.
	Insert(ctx context.Context, project *Project) (*Project, error)
	// Delete is a method for deleting project by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating project entity.
	Update(ctx context.Context, project *Project) error
	// List returns paginated projects, created before provided timestamp.
	List(ctx context.Context, offset int64, limit int, before time.Time) (ProjectsPage, error)
	// ListByOwnerID is a method for querying all projects from the database by ownerID. It also includes the number of members for each project.
	ListByOwnerID(ctx context.Context, userID uuid.UUID, cursor ProjectsCursor) (ProjectsPage, error)

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

	// UpdateDefaultPlacement is a method to update the project's default placement for new segments.
	UpdateDefaultPlacement(ctx context.Context, id uuid.UUID, placement storj.PlacementConstraint) error
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
	Paid memory.Size `help:"the default paid-tier storage usage limit" default:"25.00TB" testDefault:"25.00 GB"`
}

// BandwidthLimitConfig is a configuration struct for default bandwidth per-project usage limits.
type BandwidthLimitConfig struct {
	Free memory.Size `help:"the default free-tier bandwidth usage limit" default:"25.00GB"  testDefault:"25.00 GB"`
	Paid memory.Size `help:"the default paid-tier bandwidth usage limit" default:"100.00TB" testDefault:"25.00 GB"`
}

// SegmentLimitConfig is a configuration struct for default segments per-project usage limits.
type SegmentLimitConfig struct {
	Free int64 `help:"the default free-tier segment usage limit" default:"10000"`
	Paid int64 `help:"the default paid-tier segment usage limit" default:"100000000"`
}

// ProjectLimitConfig is a configuration struct for default project limits.
type ProjectLimitConfig struct {
	Free int `help:"the default free-tier project limit" default:"1"`
	Paid int `help:"the default paid-tier project limit" default:"3"`
}

// Project is a database object that describes Project entity.
type Project struct {
	ID       uuid.UUID `json:"id"`
	PublicID uuid.UUID `json:"publicId"`

	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserAgent   []byte    `json:"userAgent"`
	OwnerID     uuid.UUID `json:"ownerId"`
	MaxBuckets  *int      `json:"maxBuckets"`
	CreatedAt   time.Time `json:"createdAt"`
	MemberCount int       `json:"memberCount"`

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

	DefaultPlacement          storj.PlacementConstraint `json:"defaultPlacement"`
	DefaultVersioning         DefaultVersioning         `json:"defaultVersioning"`
	PromptedForVersioningBeta bool                      `json:"-"`
	PassphraseEnc             []byte                    `json:"-"`
	PathEncryption            *bool                     `json:"-"`
}

// UpsertProjectInfo holds data needed to create/update Project.
type UpsertProjectInfo struct {
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	StorageLimit   memory.Size `json:"storageLimit"`
	BandwidthLimit memory.Size `json:"bandwidthLimit"`

	// these fields are only used for inserts and ignored for updates
	CreatedAt        time.Time `json:"createdAt"`
	ManagePassphrase bool      `json:"managePassphrase"`
}

// ProjectInfo holds data sent via user facing http endpoints.
type ProjectInfo struct {
	ID               uuid.UUID         `json:"id"`
	Name             string            `json:"name"`
	OwnerID          uuid.UUID         `json:"ownerId"`
	Description      string            `json:"description"`
	MemberCount      int               `json:"memberCount"`
	CreatedAt        time.Time         `json:"createdAt"`
	EdgeURLOverrides *EdgeURLOverrides `json:"edgeURLOverrides,omitempty"`
	StorageUsed      int64             `json:"storageUsed"`
	BandwidthUsed    int64             `json:"bandwidthUsed"`
	Versioning       DefaultVersioning `json:"versioning"`
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

// ProjectsPage returns paginated projects,
// providing next offset if there are more projects
// to retrieve.
type ProjectsPage struct {
	Projects   []Project
	Next       bool
	NextOffset int64

	Limit  int
	Offset int64

	PageCount   int
	CurrentPage int
	TotalCount  int64
}

// ProjectInfoPage is similar to ProjectsPage
// except the Projects field is ProjectInfo and is sent over HTTP API.
type ProjectInfoPage struct {
	Projects []ProjectInfo `json:"projects"`

	Limit  int   `json:"limit"`
	Offset int64 `json:"offset"`

	PageCount   int   `json:"pageCount"`
	CurrentPage int   `json:"currentPage"`
	TotalCount  int64 `json:"totalCount"`
}

// LimitRequestInfo holds data needed to request limit increase.
type LimitRequestInfo struct {
	LimitType    string      `json:"limitType"`
	CurrentLimit memory.Size `json:"currentLimit"`
	DesiredLimit memory.Size `json:"desiredLimit"`
}

// ProjectConfig holds config for available "features" for a project.
type ProjectConfig struct {
	VersioningUIEnabled     bool   `json:"versioningUIEnabled"`
	PromptForVersioningBeta bool   `json:"promptForVersioningBeta"`
	Passphrase              string `json:"passphrase,omitempty"`
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

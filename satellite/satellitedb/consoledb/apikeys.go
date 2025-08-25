// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/lrucache"
)

// ensures that apikeys implements console.APIKeys.
var _ console.APIKeys = (*apikeys)(nil)

type projectApiKeyRow = dbx.ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Project_UserSpecifiedUsageLimit_Project_UserSpecifiedBandwidthLimit_Row

// apikeys is an implementation of satellite.APIKeys.
type apikeys struct {
	db   dbx.DriverMethods
	lru  *lrucache.ExpiringLRUOf[*projectApiKeyRow]
	impl dbutil.Implementation
}

// GetPagedByProjectID retrieves API keys for a given projectID and cursor.
func (keys *apikeys) GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor console.APIKeyCursor, ignoredNamePrefix string) (page *console.APIKeyPage, err error) {
	defer mon.Task()(&ctx)(&err)

	search := strings.ToLower("%" + strings.ReplaceAll(cursor.Search, " ", "%") + "%")

	if cursor.Limit == 0 {
		return nil, console.ErrAPIKeyRequest.New("limit cannot be 0")
	}

	if cursor.Page == 0 {
		return nil, console.ErrAPIKeyRequest.New("page cannot be 0")
	}

	page = &console.APIKeyPage{
		Search:         cursor.Search,
		Limit:          cursor.Limit,
		Offset:         uint64((cursor.Page - 1) * cursor.Limit),
		Order:          cursor.Order,
		OrderDirection: cursor.OrderDirection,
	}

	// This expression hides emails of ex‐members.
	emailExpr := "CASE WHEN pm.member_id IS NOT NULL THEN u.email ELSE '' END"

	whereClause := `
      WHERE ak.project_id = ?
        AND (
          LOWER(ak.name) LIKE ?
          OR LOWER(` + emailExpr + `) LIKE ?
        )
    `
	if ignoredNamePrefix != "" {
		whereClause += " AND ak.name NOT LIKE '" + ignoredNamePrefix + "%' "
	}

	countQuery := keys.db.Rebind(`
		SELECT COUNT(*)
		FROM api_keys ak
		LEFT JOIN users u
			ON u.id = ak.created_by
		LEFT JOIN project_members pm
			ON pm.project_id = ak.project_id
		AND pm.member_id = ak.created_by
    ` + whereClause)

	err = keys.db.QueryRowContext(ctx,
		countQuery,
		projectID[:], search, search,
	).Scan(&page.TotalCount)
	if err != nil {
		return nil, err
	}

	if page.TotalCount == 0 {
		return page, nil
	}
	if page.Offset > page.TotalCount-1 {
		return nil, console.ErrAPIKeyRequest.New("page is out of range")
	}

	repoundQuery := keys.db.Rebind(`
		SELECT
			ak.id,
			ak.project_id,
			ak.name,
			ak.user_agent,
			ak.created_at,
			ak.version,
			p.public_id AS project_public_id,
			` + emailExpr + ` AS creator_email
		FROM api_keys ak
		JOIN projects p
			ON p.id = ak.project_id
		LEFT JOIN users u
			ON u.id = ak.created_by
		LEFT JOIN project_members pm
			ON pm.project_id = ak.project_id
			AND pm.member_id = ak.created_by
    	` + whereClause + apikeySortClause(cursor.Order, cursor.OrderDirection) + ` LIMIT ? OFFSET ?`,
	)

	rows, err := keys.db.QueryContext(
		ctx,
		repoundQuery,
		projectID[:],
		search,
		search,
		page.Limit,
		page.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		ak := console.APIKeyInfo{}

		err = rows.Scan(&ak.ID, &ak.ProjectID, &ak.Name, &ak.UserAgent, &ak.CreatedAt, &ak.Version, &ak.ProjectPublicID, &ak.CreatorEmail)
		if err != nil {
			return nil, err
		}

		page.APIKeys = append(page.APIKeys, ak)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	page.Order = cursor.Order
	page.CurrentPage = cursor.Page
	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	return page, err
}

// Get implements satellite.APIKeys.
func (keys *apikeys) Get(ctx context.Context, id uuid.UUID) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKey, err := keys.db.Get_ApiKey_Project_PublicId_By_ApiKey_Id(ctx, dbx.ApiKey_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXApiKeyProjectPublicIdRow(ctx, dbKey)
}

// GetByHead implements satellite.APIKeys.
func (keys *apikeys) GetByHead(ctx context.Context, head []byte) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	dbKey, err := keys.lru.Get(ctx, string(head), func() (*dbx.ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Project_UserSpecifiedUsageLimit_Project_UserSpecifiedBandwidthLimit_Row, error) {
		return keys.db.Get_ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Project_UserSpecifiedUsageLimit_Project_UserSpecifiedBandwidthLimit_By_ApiKey_Head(ctx, dbx.ApiKey_Head(head))
	})
	if err != nil {
		return nil, err
	}
	return fromDBXApiKey_ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Project_UserSpecifiedUsageLimit_Project_UserSpecifiedBandwidthLimit_Row(ctx, dbKey)
}

// GetByNameAndProjectID implements satellite.APIKeys.
func (keys *apikeys) GetByNameAndProjectID(ctx context.Context, name string, projectID uuid.UUID) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKey, err := keys.db.Get_ApiKey_Project_PublicId_By_ApiKey_Name_And_ApiKey_ProjectId(ctx,
		dbx.ApiKey_Name(name),
		dbx.ApiKey_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXApiKeyProjectPublicIdRow(ctx, dbKey)
}

// GetAllNamesByProjectID implements satellite.APIKeys.
func (keys *apikeys) GetAllNamesByProjectID(ctx context.Context, projectID uuid.UUID) ([]string, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	query := keys.db.Rebind(`
		SELECT ak.name
		FROM api_keys ak
		WHERE ak.project_id = ?
		` + apikeySortClause(console.KeyName, console.Ascending),
	)

	rows, err := keys.db.QueryContext(ctx, query, projectID[:])
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	names := []string{}
	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}

		names = append(names, name)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return names, nil
}

// Create implements satellite.APIKeys.
func (keys *apikeys) Create(ctx context.Context, head []byte, info console.APIKeyInfo) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	optional := dbx.ApiKey_Create_Fields{
		Version: dbx.ApiKey_Version(uint(info.Version)),
	}
	if info.UserAgent != nil {
		optional.UserAgent = dbx.ApiKey_UserAgent(info.UserAgent)
	}
	if !info.CreatedBy.IsZero() {
		optional.CreatedBy = dbx.ApiKey_CreatedBy(info.CreatedBy[:])
	}

	apiKey, err := keys.db.Create_ApiKey(
		ctx,
		dbx.ApiKey_Id(id[:]),
		dbx.ApiKey_ProjectId(info.ProjectID[:]),
		dbx.ApiKey_Head(head),
		dbx.ApiKey_Name(info.Name),
		dbx.ApiKey_Secret(info.Secret),
		optional,
	)

	if err != nil {
		return nil, err
	}

	return apiKeyToAPIKeyInfo(ctx, apiKey)
}

// Update implements satellite.APIKeys.
func (keys *apikeys) Update(ctx context.Context, key console.APIKeyInfo) (err error) {
	defer mon.Task()(&ctx)(&err)
	return keys.db.UpdateNoReturn_ApiKey_By_Id(
		ctx,
		dbx.ApiKey_Id(key.ID[:]),
		dbx.ApiKey_Update_Fields{
			Name: dbx.ApiKey_Name(key.Name),
		},
	)
}

// Delete implements satellite.APIKeys.
func (keys *apikeys) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = keys.db.Delete_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	return err
}

// DeleteMultiple implements satellite.APIKeys.
func (keys *apikeys) DeleteMultiple(ctx context.Context, ids []uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	switch keys.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		query := `DELETE FROM api_keys WHERE id = ANY($1)`
		_, err = keys.db.ExecContext(ctx, query, pgutil.UUIDArray(ids))
	case dbutil.Spanner:
		query := `DELETE FROM api_keys WHERE id IN UNNEST(?)`
		_, err = keys.db.ExecContext(ctx, query, uuidsToBytesArray(ids))
	default:
		return errs.New("unsupported database dialect: %s", keys.impl)
	}
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return err
}

// DeleteAllByProjectID deletes all APIKeyInfos from store by given projectID.
func (keys *apikeys) DeleteAllByProjectID(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = keys.db.Delete_ApiKey_By_ProjectId(ctx, dbx.ApiKey_ProjectId(id[:]))
	return err
}

// DeleteExpiredByNamePrefix deletes expired APIKeyInfo from store by key name prefix.
func (keys *apikeys) DeleteExpiredByNamePrefix(ctx context.Context, lifetime time.Duration, prefix string, asOfSystemTimeInterval time.Duration, pageSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pageSize <= 0 {
		return Error.New("expected page size to be positive; got %d", pageSize)
	}

	type keyInfo struct {
		id        uuid.UUID
		createdAt time.Time
	}

	var pageCursor uuid.UUID
	var toBeDeleted []uuid.UUID
	found := make([]keyInfo, pageSize)
	aost := keys.db.AsOfSystemInterval(asOfSystemTimeInterval)
	now := time.Now()

	cursorQuery := `
		SELECT id FROM api_keys
	` + aost + `
		WHERE id > ? AND api_keys.name LIKE
	'` + prefix + `%'
		ORDER BY id LIMIT 1
	`
	selectQuery := `
		SELECT id, created_at FROM api_keys
	` + aost + `
		WHERE id >= ? AND api_keys.name LIKE
	'` + prefix + `%'
		ORDER BY id LIMIT ?
	`

	for {
		// Select the ID beginning this page of records
		err = keys.db.QueryRowContext(ctx, keys.db.Rebind(cursorQuery), pageCursor).Scan(&pageCursor)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		// Select page of records
		rows, err := keys.db.QueryContext(ctx, keys.db.Rebind(selectQuery), pageCursor, pageSize)
		if err != nil {
			return Error.Wrap(err)
		}

		var i int
		for i = 0; rows.Next(); i++ {
			key := keyInfo{}

			err = rows.Scan(&key.id, &key.createdAt)
			if err != nil {
				return Error.Wrap(err)
			}

			found[i] = key

			if now.After(key.createdAt.Add(lifetime)) {
				toBeDeleted = append(toBeDeleted, key.id)
			}
		}
		if err = errs.Combine(rows.Err(), rows.Close()); err != nil {
			return Error.Wrap(err)
		}

		// Delete all expired keys in the page
		if len(toBeDeleted) != 0 {
			switch keys.impl {
			case dbutil.Cockroach, dbutil.Postgres:
				query := `DELETE FROM api_keys WHERE id = ANY($1)`
				_, err = keys.db.ExecContext(ctx, query, pgutil.UUIDArray(toBeDeleted))
			case dbutil.Spanner:
				query := `DELETE FROM api_keys WHERE id IN UNNEST(?)`
				_, err = keys.db.ExecContext(ctx, query, uuidsToBytesArray(toBeDeleted))
			default:
				return errs.New("unsupported database dialect: %s", keys.impl)
			}

			if err != nil {
				return Error.Wrap(err)
			}
		}

		if i < pageSize {
			return nil
		}

		// Advance the cursor to the next page
		pageCursor = found[i-1].id
	}
}

func apiKeyToAPIKeyInfo(ctx context.Context, key *dbx.ApiKey) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := uuid.FromBytes(key.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := uuid.FromBytes(key.ProjectId)
	if err != nil {
		return nil, err
	}

	var createdBy uuid.UUID
	if key.CreatedBy != nil {
		createdBy, err = uuid.FromBytes(key.CreatedBy)
		if err != nil {
			return nil, err
		}
	}

	result := &console.APIKeyInfo{
		ID:        id,
		ProjectID: projectID,
		CreatedBy: createdBy,
		Name:      key.Name,
		CreatedAt: key.CreatedAt,
		Head:      key.Head,
		Secret:    key.Secret,
		Version:   macaroon.APIKeyVersion(key.Version),
	}

	if key.UserAgent != nil {
		result.UserAgent = key.UserAgent
	}

	return result, nil
}

func fromDBXApiKeyProjectPublicIdRow(ctx context.Context, row *dbx.ApiKey_Project_PublicId_Row) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := apiKeyToAPIKeyInfo(ctx, &row.ApiKey)
	if err != nil {
		return nil, err
	}
	result.ProjectPublicID, err = uuid.FromBytes(row.Project_PublicId)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func fromDBXApiKey_ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Project_UserSpecifiedUsageLimit_Project_UserSpecifiedBandwidthLimit_Row(ctx context.Context, row *dbx.ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_RateLimitHead_Project_BurstLimitHead_Project_RateLimitGet_Project_BurstLimitGet_Project_RateLimitPut_Project_BurstLimitPut_Project_RateLimitList_Project_BurstLimitList_Project_RateLimitDel_Project_BurstLimitDel_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Project_UserSpecifiedUsageLimit_Project_UserSpecifiedBandwidthLimit_Row) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := apiKeyToAPIKeyInfo(ctx, &row.ApiKey)
	if err != nil {
		return nil, err
	}
	result.ProjectPublicID, err = uuid.FromBytes(row.Project_PublicId)
	if err != nil {
		return nil, err
	}
	result.ProjectRateLimit = row.Project_RateLimit
	result.ProjectBurstLimit = row.Project_BurstLimit
	result.ProjectRateLimitHead = row.Project_RateLimitHead
	result.ProjectBurstLimitHead = row.Project_BurstLimitHead
	result.ProjectRateLimitGet = row.Project_RateLimitGet
	result.ProjectBurstLimitGet = row.Project_BurstLimitGet
	result.ProjectRateLimitPut = row.Project_RateLimitPut
	result.ProjectBurstLimitPut = row.Project_BurstLimitPut
	result.ProjectRateLimitList = row.Project_RateLimitList
	result.ProjectBurstLimitList = row.Project_BurstLimitList
	result.ProjectRateLimitDelete = row.Project_RateLimitDel
	result.ProjectBurstLimitDelete = row.Project_BurstLimitDel

	result.ProjectBandwidthLimit = row.Project_BandwidthLimit
	if row.Project_UserSpecifiedBandwidthLimit != nil {
		result.ProjectBandwidthLimit = row.Project_UserSpecifiedBandwidthLimit
	}
	result.ProjectStorageLimit = row.Project_UsageLimit
	if row.Project_UserSpecifiedUsageLimit != nil {
		result.ProjectStorageLimit = row.Project_UserSpecifiedUsageLimit
	}
	result.ProjectSegmentsLimit = row.Project_SegmentLimit

	return result, nil
}

// apikeySortClause returns what ORDER BY clause should be used when sorting API key results.
func apikeySortClause(order console.APIKeyOrder, direction console.OrderDirection) string {
	dirStr := "ASC"
	if direction == console.Descending {
		dirStr = "DESC"
	}

	switch order {
	case console.CreationDate:
		return " ORDER BY ak.created_at " + dirStr + ", ak.name, ak.project_id "
	case console.KeyCreatorEmail:
		// we COALESCE to '' so NULL emails sort consistently,
		// and LOWER() so sorting is case‑insensitive.
		return " ORDER BY LOWER(COALESCE(u.email, '')) " + dirStr + ", ak.name, ak.project_id "
	default:
		return " ORDER BY LOWER(ak.name) " + dirStr + ", ak.name, ak.project_id "
	}
}

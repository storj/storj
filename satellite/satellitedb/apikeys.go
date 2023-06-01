// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/lrucache"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that apikeys implements console.APIKeys.
var _ console.APIKeys = (*apikeys)(nil)

// apikeys is an implementation of satellite.APIKeys.
type apikeys struct {
	methods dbx.Methods
	lru     *lrucache.ExpiringLRUOf[*dbx.ApiKey_Project_PublicId_Row]
	db      *satelliteDB
}

func (keys *apikeys) GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor console.APIKeyCursor) (akp *console.APIKeyPage, err error) {
	defer mon.Task()(&ctx)(&err)

	search := "%" + strings.ReplaceAll(cursor.Search, " ", "%") + "%"

	if cursor.Limit > 50 {
		cursor.Limit = 50
	}

	if cursor.Limit == 0 {
		return nil, console.ErrAPIKeyRequest.New("limit cannot be 0")
	}

	if cursor.Page == 0 {
		return nil, console.ErrAPIKeyRequest.New("page cannot be 0")
	}

	page := &console.APIKeyPage{
		Search:         cursor.Search,
		Limit:          cursor.Limit,
		Offset:         uint64((cursor.Page - 1) * cursor.Limit),
		Order:          cursor.Order,
		OrderDirection: cursor.OrderDirection,
	}

	countQuery := keys.db.Rebind(`
		SELECT COUNT(*)
		FROM api_keys ak
		WHERE ak.project_id = ?
		AND lower(ak.name) LIKE ?
	`)

	countRow := keys.db.QueryRowContext(ctx,
		countQuery,
		projectID[:],
		strings.ToLower(search))

	err = countRow.Scan(&page.TotalCount)
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
		SELECT ak.id, ak.project_id, ak.name, ak.user_agent, ak.created_at, p.public_id
		FROM api_keys ak, projects p
		WHERE ak.project_id = ?
		AND ak.project_id = p.id
		AND lower(ak.name) LIKE ?
		ORDER BY ` + sanitizedAPIKeyOrderColumnName(cursor.Order) + `
		` + sanitizeOrderDirectionName(page.OrderDirection) + `
		LIMIT ? OFFSET ?`)

	rows, err := keys.db.QueryContext(ctx,
		repoundQuery,
		projectID[:],
		strings.ToLower(search),
		page.Limit,
		page.Offset)

	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var apiKeys []console.APIKeyInfo
	for rows.Next() {
		ak := console.APIKeyInfo{}

		err = rows.Scan(&ak.ID, &ak.ProjectID, &ak.Name, &ak.UserAgent, &ak.CreatedAt, &ak.ProjectPublicID)
		if err != nil {
			return nil, err
		}

		apiKeys = append(apiKeys, ak)
	}

	page.APIKeys = apiKeys
	page.Order = cursor.Order

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.CurrentPage = cursor.Page

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return page, err
}

// Get implements satellite.APIKeys.
func (keys *apikeys) Get(ctx context.Context, id uuid.UUID) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKey, err := keys.methods.Get_ApiKey_Project_PublicId_By_ApiKey_Id(ctx, dbx.ApiKey_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(ctx, dbKey)
}

// GetByHead implements satellite.APIKeys.
func (keys *apikeys) GetByHead(ctx context.Context, head []byte) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	dbKey, err := keys.lru.Get(ctx, string(head), func() (*dbx.ApiKey_Project_PublicId_Row, error) {
		return keys.methods.Get_ApiKey_Project_PublicId_By_ApiKey_Head(ctx, dbx.ApiKey_Head(head))
	})
	if err != nil {
		return nil, err
	}
	return fromDBXAPIKey(ctx, dbKey)
}

// GetByNameAndProjectID implements satellite.APIKeys.
func (keys *apikeys) GetByNameAndProjectID(ctx context.Context, name string, projectID uuid.UUID) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKey, err := keys.methods.Get_ApiKey_Project_PublicId_By_ApiKey_Name_And_ApiKey_ProjectId(ctx,
		dbx.ApiKey_Name(name),
		dbx.ApiKey_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(ctx, dbKey)
}

// Create implements satellite.APIKeys.
func (keys *apikeys) Create(ctx context.Context, head []byte, info console.APIKeyInfo) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	optional := dbx.ApiKey_Create_Fields{}
	if info.UserAgent != nil {
		optional.UserAgent = dbx.ApiKey_UserAgent(info.UserAgent)
	}

	_, err = keys.methods.Create_ApiKey(
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

	return keys.Get(ctx, id)
}

// Update implements satellite.APIKeys.
func (keys *apikeys) Update(ctx context.Context, key console.APIKeyInfo) (err error) {
	defer mon.Task()(&ctx)(&err)
	return keys.methods.UpdateNoReturn_ApiKey_By_Id(
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
	_, err = keys.methods.Delete_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	return err
}

// fromDBXAPIKey converts dbx.ApiKey to satellite.APIKeyInfo.
func fromDBXAPIKey(ctx context.Context, row *dbx.ApiKey_Project_PublicId_Row) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	key := &row.ApiKey
	id, err := uuid.FromBytes(key.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := uuid.FromBytes(key.ProjectId)
	if err != nil {
		return nil, err
	}
	projectPublicID, err := uuid.FromBytes(row.Project_PublicId)
	if err != nil {
		return nil, err
	}

	result := &console.APIKeyInfo{
		ID:              id,
		ProjectID:       projectID,
		ProjectPublicID: projectPublicID,
		Name:            key.Name,
		CreatedAt:       key.CreatedAt,
		Head:            key.Head,
		Secret:          key.Secret,
	}

	if key.UserAgent != nil {
		result.UserAgent = key.UserAgent
	}

	return result, nil
}

// sanitizedAPIKeyOrderColumnName return valid order by column.
func sanitizedAPIKeyOrderColumnName(pmo console.APIKeyOrder) string {
	if pmo == 2 {
		return "ak.created_at"
	}

	return "lower(ak.name)"
}

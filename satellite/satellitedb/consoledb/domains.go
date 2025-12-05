// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that domains implements console.Domains.
var _ console.Domains = (*domains)(nil)

// implementation of Domains interface repository using spacemonkeygo/dbx orm.
type domains struct {
	db dbx.DriverMethods
}

// Create implements satellite.Domains method to create new Domain.
func (d *domains) Create(ctx context.Context, data console.Domain) (_ *console.Domain, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxDomain, err := d.db.Create_Domain(
		ctx,
		dbx.Domain_Subdomain(data.Subdomain),
		dbx.Domain_ProjectId(data.ProjectID[:]),
		dbx.Domain_Prefix(data.Prefix),
		dbx.Domain_AccessId(data.AccessID),
		dbx.Domain_CreatedBy(data.CreatedBy[:]),
	)
	if err != nil {
		if dbx.IsConstraintError(err) {
			return nil, console.ErrSubdomainAlreadyExists.New("")
		}
		return nil, err
	}

	return domainFromDBX(ctx, dbxDomain)
}

// Delete implements satellite.Domains delete domain by project ID and subdomain method.
func (d *domains) Delete(ctx context.Context, projectID uuid.UUID, subdomain string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = d.db.Delete_Domain_By_ProjectId_And_Subdomain(ctx, dbx.Domain_ProjectId(projectID[:]), dbx.Domain_Subdomain(subdomain))
	return err
}

// DeleteAllByProjectID implements satellite.Domains delete all domains by project ID method.
func (d *domains) DeleteAllByProjectID(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = d.db.Delete_Domain_By_ProjectId(ctx, dbx.Domain_ProjectId(projectID[:]))
	return err
}

// GetByProjectIDAndSubdomain implements satellite.Domains get domain by project ID and subdomain method.
func (d *domains) GetByProjectIDAndSubdomain(ctx context.Context, projectID uuid.UUID, subdomain string) (_ *console.Domain, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxDomain, err := d.db.Get_Domain_By_ProjectId_And_Subdomain(
		ctx,
		dbx.Domain_ProjectId(projectID[:]),
		dbx.Domain_Subdomain(subdomain),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, console.ErrNoSubdomain.New("")
		}
		return nil, err
	}

	return domainFromDBX(ctx, dbxDomain)
}

// GetAllDomainNamesByProjectID implements satellite.Domains get all domain names by project ID method.
func (d *domains) GetAllDomainNamesByProjectID(ctx context.Context, projectID uuid.UUID) (names []string, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := d.db.All_Domain_Subdomain_By_ProjectId(ctx, dbx.Domain_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	names = make([]string, 0, len(rows))

	for _, row := range rows {
		if row == nil {
			return nil, errs.New("nil row in database result")
		}

		names = append(names, row.Subdomain)
	}

	return names, nil
}

// GetPagedByProjectID implements satellite.Domains get domains by project ID and cursor method.
func (d *domains) GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor console.DomainCursor) (page *console.DomainPage, err error) {
	defer mon.Task()(&ctx)(&err)

	search := "%" + strings.ReplaceAll(cursor.Search, " ", "%") + "%"

	if cursor.Limit == 0 {
		return nil, errs.New("limit cannot be 0")
	}
	if cursor.Page == 0 {
		return nil, errs.New("page cannot be 0")
	}

	page = &console.DomainPage{
		Search:         cursor.Search,
		Limit:          cursor.Limit,
		Offset:         uint64((cursor.Page - 1) * cursor.Limit),
		Order:          cursor.Order,
		OrderDirection: cursor.OrderDirection,
	}

	countQuery := d.db.Rebind(`
        SELECT COUNT(*)
        FROM domains d
        WHERE d.project_id = ?
        AND lower(d.subdomain) LIKE ?
    `)

	countRow := d.db.QueryRowContext(ctx,
		countQuery,
		projectID[:],
		strings.ToLower(search),
	)

	err = countRow.Scan(&page.TotalCount)
	if err != nil {
		return nil, err
	}
	if page.TotalCount == 0 {
		return page, nil
	}
	if page.Offset > page.TotalCount-1 {
		return nil, errs.New("page is out of range")
	}

	query := d.db.Rebind(`
        SELECT 
            d.project_id,
            d.subdomain,
            d.prefix,
            d.access_id,
            d.created_by,
            d.created_at,
            p.public_id
        FROM domains d, projects p
        WHERE d.project_id = ?
        AND d.project_id = p.id
        AND lower(d.subdomain) LIKE ?
    ` + domainSortClause(cursor.Order, page.OrderDirection) + `
        LIMIT ? OFFSET ?
    `)

	rows, err := d.db.QueryContext(ctx,
		query,
		projectID[:],
		strings.ToLower(search),
		page.Limit,
		page.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var domainsList []console.Domain
	for rows.Next() {
		var dom console.Domain
		err = rows.Scan(&dom.ProjectID, &dom.Subdomain, &dom.Prefix, &dom.AccessID, &dom.CreatedBy, &dom.CreatedAt, &dom.ProjectPublicID)
		if err != nil {
			return nil, err
		}
		domainsList = append(domainsList, dom)
	}
	page.Domains = domainsList
	page.Order = cursor.Order

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}
	page.CurrentPage = cursor.Page

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return page, nil
}

// domainFromDBX is used for creating Domain entity from autogenerated dbx.Domain struct.
func domainFromDBX(ctx context.Context, domain *dbx.Domain) (_ *console.Domain, err error) {
	defer mon.Task()(&ctx)(&err)

	if domain == nil {
		return nil, errs.New("domain parameter is nil")
	}

	projectID, err := uuid.FromBytes(domain.ProjectId)
	if err != nil {
		return nil, err
	}

	createdBy, err := uuid.FromBytes(domain.CreatedBy)
	if err != nil {
		return nil, err
	}

	return &console.Domain{
		ProjectID: projectID,
		CreatedBy: createdBy,
		Subdomain: domain.Subdomain,
		Prefix:    domain.Prefix,
		AccessID:  domain.AccessId,
		CreatedAt: domain.CreatedAt,
	}, nil
}

// domainSortClause returns the ORDER BY clause for domain queries based on the given order and direction.
func domainSortClause(order console.DomainOrder, direction console.OrderDirection) string {
	dirStr := "ASC"
	if direction == console.Descending {
		dirStr = "DESC"
	}

	// Use the console.CreationDateOrder if specified.
	if order == console.CreationDateOrder {
		return "ORDER BY d.created_at " + dirStr + ", d.subdomain, d.project_id"
	}
	// Default to sorting by console.SubdomainOrder.
	return "ORDER BY LOWER(d.subdomain) " + dirStr + ", d.subdomain, d.project_id"
}

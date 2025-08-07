// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/private/slices2"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

// ensures that projectMembers implements console.ProjectMembers.
var _ console.ProjectMembers = (*projectMembers)(nil)

// ProjectMembers exposes db to manage ProjectMembers table in database.
type projectMembers struct {
	db   dbx.DriverMethods
	impl dbutil.Implementation
}

// GetByMemberID is a method for querying project member from the database by memberID.
func (pm *projectMembers) GetByMemberID(ctx context.Context, memberID uuid.UUID) (_ []console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	projectMembersDbx, err := pm.db.All_ProjectMember_By_MemberId(ctx, dbx.ProjectMember_MemberId(memberID[:]))
	if err != nil {
		return nil, err
	}

	return projectMembersFromDbxSlice(ctx, projectMembersDbx)
}

// GetByMemberIDAndProjectID is a method for querying project member from the database by memberID and projectID.
func (pm *projectMembers) GetByMemberIDAndProjectID(ctx context.Context, memberID, projectID uuid.UUID) (*console.ProjectMember, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	projectMember, err := pm.db.Get_ProjectMember_By_MemberId_And_ProjectId(ctx,
		dbx.ProjectMember_MemberId(memberID[:]),
		dbx.ProjectMember_ProjectId(projectID[:]),
	)
	if err != nil {
		return nil, err
	}

	return projectMemberFromDBX(ctx, projectMember)
}

func (pm *projectMembers) UpdateRole(ctx context.Context, memberID, projectID uuid.UUID, newRole console.ProjectMemberRole) (_ *console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	projectMember, err := pm.db.Update_ProjectMember_By_MemberId_And_ProjectId(ctx,
		dbx.ProjectMember_MemberId(memberID[:]),
		dbx.ProjectMember_ProjectId(projectID[:]),
		dbx.ProjectMember_Update_Fields{
			Role: dbx.ProjectMember_Role(int(newRole)),
		},
	)
	if err != nil {
		return nil, err
	}

	return projectMemberFromDBX(ctx, projectMember)
}

// GetTotalCountByProjectID is a method for getting total count of project members by projectID.
func (pm *projectMembers) GetTotalCountByProjectID(ctx context.Context, projectID uuid.UUID) (count uint64, err error) {
	defer mon.Task()(&ctx)(&err)

	countQuery := pm.db.Rebind(`
		SELECT COUNT(*)
		FROM project_members
		WHERE project_id = ?
	`)

	err = pm.db.QueryRowContext(ctx, countQuery, projectID[:]).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetPagedWithInvitationsByProjectID is a method for querying project members and invitations from the database by projectID, offset and limit.
func (pm *projectMembers) GetPagedWithInvitationsByProjectID(ctx context.Context, projectID uuid.UUID, cursor console.ProjectMembersCursor) (_ *console.ProjectMembersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	search := "%" + strings.ReplaceAll(cursor.Search, " ", "%") + "%"

	if cursor.Limit == 0 {
		return nil, errs.New("limit cannot be 0")
	}

	if cursor.Page == 0 {
		return nil, errs.New("page cannot be 0")
	}

	page := &console.ProjectMembersPage{
		Search:         cursor.Search,
		Limit:          cursor.Limit,
		Offset:         uint64((cursor.Page - 1) * cursor.Limit),
		Order:          cursor.Order,
		OrderDirection: cursor.OrderDirection,
	}

	var countRow *sql.Row

	switch pm.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		countQuery := `
		SELECT (
			SELECT COUNT(*)
			FROM project_members pm
			INNER JOIN users u ON pm.member_id = u.id
			WHERE pm.project_id = $1
			AND (
				u.email ILIKE $2 OR
				u.full_name ILIKE $2 OR
				u.short_name ILIKE $2
			)
		) + (
			SELECT COUNT(*)
			FROM project_invitations
			WHERE project_id = $1
			AND email ILIKE $2
		)`

		countRow = pm.db.QueryRowContext(ctx,
			countQuery,
			projectID[:],
			search)

	case dbutil.Spanner:
		countQuery := `
			WITH pm_cte AS (
				SELECT COUNT(*) AS cnt
				FROM project_members pm
				INNER JOIN users u ON pm.member_id = u.id
				WHERE pm.project_id = @project_id
				AND (
						lower(u.email) LIKE lower(@search) OR
						lower(u.full_name) LIKE lower(@search) OR
						lower(u.short_name) LIKE lower(@search)
				)
			),
			pi_cte AS (
				SELECT COUNT(*) as cnt
				FROM project_invitations
				WHERE project_id = @project_id
				AND lower(email) LIKE lower(@search)
			)
			SELECT pi_cte.cnt + pm_cte.cnt FROM pm_cte,pi_cte;`

		countRow = pm.db.QueryRowContext(ctx,
			countQuery,
			sql.Named("project_id", projectID.Bytes()),
			sql.Named("search", search))

	default:
		return nil, Error.New("unsupported database: %v", pm.impl)
	}

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

	var rows tagsql.Rows

	switch pm.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		membersQuery := `
		SELECT member_id, project_id, role, created_at, email, inviter_id FROM (
			(
				SELECT pm.member_id, pm.project_id, pm.role, pm.created_at, u.email, u.full_name, NULL as inviter_id
				FROM project_members pm
				INNER JOIN users u ON pm.member_id = u.id
				WHERE pm.project_id = $1
				AND (
					u.email ILIKE $2 OR
					u.full_name ILIKE $2 OR
					u.short_name ILIKE $2
				)
			) UNION ALL (
				SELECT NULL as member_id, project_id, 1 as role, created_at, LOWER(email) as email, LOWER(SPLIT_PART(email, '@', 1)) as full_name, inviter_id
				FROM project_invitations pi
				WHERE project_id = $1
				AND email ILIKE $2
			)
		) results
		` + projectMembersSortClause(cursor.Order, page.OrderDirection) + `
		LIMIT $3 OFFSET $4`

		rows, err = pm.db.QueryContext(ctx,
			membersQuery,
			projectID[:],
			search,
			page.Limit,
			page.Offset,
		)
	case dbutil.Spanner:
		membersQuery := `SELECT member_id, project_id, role, created_at, email, inviter_id FROM (
                        (
                                SELECT pm.member_id, pm.project_id, pm.role, pm.created_at, u.email, u.full_name, NULL as inviter_id
                                FROM project_members pm
                                INNER JOIN users u ON pm.member_id = u.id
                                WHERE pm.project_id = @project_id
                                AND (
                                        LOWER(u.email) LIKE LOWER(@search) OR
                                        LOWER(u.full_name) LIKE LOWER(@search) OR
                                        LOWER(u.short_name) LIKE LOWER(@search)
                                )
                        ) UNION ALL (
                                SELECT NULL as member_id, project_id, 1 as role, created_at, LOWER(email) as email, LOWER(SPLIT(email, '@')[OFFSET(0)]) as full_name, inviter_id
                                FROM project_invitations pi
                                WHERE project_id = @project_id
                                AND LOWER(email) LIKE LOWER(@search)
                        )
                ) results
				` + projectMembersSortClause(cursor.Order, page.OrderDirection) + `
                LIMIT @limit OFFSET @offset`

		rows, err = pm.db.QueryContext(ctx, membersQuery,
			sql.Named("project_id", projectID),
			sql.Named("search", search),
			sql.Named("limit", page.Limit),
			sql.Named("offset", page.Offset),
		)
	default:
		return nil, Error.New("unsupported database: %v", pm.impl)
	}

	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var (
			memberID  uuid.NullUUID
			projectID uuid.UUID
			role      console.ProjectMemberRole
			createdAt time.Time
			email     string
			inviterID uuid.NullUUID
		)

		err = rows.Scan(
			&memberID,
			&projectID,
			&role,
			&createdAt,
			&email,
			&inviterID,
		)
		if err != nil {
			return nil, err
		}

		if memberID.Valid {
			page.ProjectMembers = append(page.ProjectMembers, console.ProjectMember{
				MemberID:  memberID.UUID,
				ProjectID: projectID,
				Role:      role,
				CreatedAt: createdAt,
			})
		} else {
			invite := console.ProjectInvitation{
				ProjectID: projectID,
				Email:     email,
				CreatedAt: createdAt,
			}
			if inviterID.Valid {
				invite.InviterID = &inviterID.UUID
			}
			page.ProjectInvitations = append(page.ProjectInvitations, invite)
		}

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.CurrentPage = cursor.Page

	return page, err
}

// Insert is a method for inserting project member into the database.
func (pm *projectMembers) Insert(ctx context.Context, memberID, projectID uuid.UUID, role console.ProjectMemberRole) (_ *console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	createdProjectMember, err := pm.db.Create_ProjectMember(ctx,
		dbx.ProjectMember_MemberId(memberID[:]),
		dbx.ProjectMember_ProjectId(projectID[:]),
		dbx.ProjectMember_Create_Fields{
			Role: dbx.ProjectMember_Role(int(role)),
		},
	)
	if err != nil {
		return nil, err
	}

	return projectMemberFromDBX(ctx, createdProjectMember)
}

// Delete is a method for deleting project member by memberID and projectID from the database.
func (pm *projectMembers) Delete(ctx context.Context, memberID, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = pm.db.Delete_ProjectMember_By_MemberId_And_ProjectId(
		ctx,
		dbx.ProjectMember_MemberId(memberID[:]),
		dbx.ProjectMember_ProjectId(projectID[:]),
	)

	return err
}

// projectMemberFromDBX is used for creating ProjectMember entity from autogenerated dbx.ProjectMember struct.
func projectMemberFromDBX(ctx context.Context, projectMember *dbx.ProjectMember) (_ *console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	if projectMember == nil {
		return nil, errs.New("projectMember parameter is nil")
	}

	memberID, err := uuid.FromBytes(projectMember.MemberId)
	if err != nil {
		return nil, err
	}

	projectID, err := uuid.FromBytes(projectMember.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.ProjectMember{
		MemberID:  memberID,
		ProjectID: projectID,
		Role:      console.ProjectMemberRole(projectMember.Role),
		CreatedAt: projectMember.CreatedAt,
	}, nil
}

// projectMembersSortClause returns what ORDER BY clause should be used when sorting project member results.
func projectMembersSortClause(order console.ProjectMemberOrder, direction console.OrderDirection) string {
	dirStr := "ASC"
	if direction == console.Descending {
		dirStr = "DESC"
	}

	switch order {
	case console.Email:
		return "ORDER BY LOWER(email) " + dirStr
	case console.Created:
		return "ORDER BY created_at " + dirStr + ", LOWER(email)"
	}
	return "ORDER BY LOWER(full_name) " + dirStr + ", LOWER(email)"
}

// projectMembersFromDbxSlice is used for creating []ProjectMember entities from autogenerated []*dbx.ProjectMember struct.
func projectMembersFromDbxSlice(ctx context.Context, projectMembersDbx []*dbx.ProjectMember) (_ []console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	rs, errors := slices2.ConvertErrs(projectMembersDbx,
		func(v *dbx.ProjectMember) (r console.ProjectMember, _ error) {
			p, err := projectMemberFromDBX(ctx, v)
			if err != nil {
				return r, err
			}
			return *p, err
		})
	return rs, errs.Combine(errors...)
}

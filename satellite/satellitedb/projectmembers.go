// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that projectMembers implements console.ProjectMembers.
var _ console.ProjectMembers = (*projectMembers)(nil)

// ProjectMembers exposes methods to manage ProjectMembers table in database.
type projectMembers struct {
	methods dbx.Methods
	db      *satelliteDB
}

// GetByMemberID is a method for querying project member from the database by memberID.
func (pm *projectMembers) GetByMemberID(ctx context.Context, memberID uuid.UUID) (_ []console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	projectMembersDbx, err := pm.methods.All_ProjectMember_By_MemberId(ctx, dbx.ProjectMember_MemberId(memberID[:]))
	if err != nil {
		return nil, err
	}

	return projectMembersFromDbxSlice(ctx, projectMembersDbx)
}

// GetByProjectID is a method for querying project members from the database by projectID, offset and limit.
// TODO: Remove once all uses have been replaced by GetPagedWithInvitationsByProjectID.
func (pm *projectMembers) GetPagedByProjectID(ctx context.Context, projectID uuid.UUID, cursor console.ProjectMembersCursor) (_ *console.ProjectMembersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	search := "%" + strings.ReplaceAll(cursor.Search, " ", "%") + "%"

	if cursor.Limit > 50 {
		cursor.Limit = 50
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

	countQuery := pm.db.Rebind(`
		SELECT COUNT(*)
		FROM project_members pm
		INNER JOIN users u ON pm.member_id = u.id
		WHERE pm.project_id = ?
		AND ( u.email LIKE ? OR
			  u.full_name LIKE ? OR
			  u.short_name LIKE ?
		)`)

	countRow := pm.db.QueryRowContext(ctx,
		countQuery,
		projectID[:],
		search,
		search,
		search)

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
	// TODO: LIKE is case-sensitive postgres, however this should be case-insensitive and possibly allow typos
	reboundQuery := pm.db.Rebind(`
		SELECT pm.*
			FROM project_members pm
				INNER JOIN users u ON pm.member_id = u.id
				WHERE pm.project_id = ?
				AND ( u.email LIKE ? OR
					u.full_name LIKE ? OR
					u.short_name LIKE ? )
					ORDER BY ` + sanitizedOrderColumnName(cursor.Order) + `
					` + sanitizeOrderDirectionName(page.OrderDirection) + `
					LIMIT ? OFFSET ?`)

	rows, err := pm.db.QueryContext(ctx,
		reboundQuery,
		projectID[:],
		search,
		search,
		search,
		page.Limit,
		page.Offset)

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	if err != nil {
		return nil, err
	}

	var projectMembers []console.ProjectMember
	for rows.Next() {
		pm := console.ProjectMember{}

		err = rows.Scan(&pm.MemberID, &pm.ProjectID, &pm.CreatedAt)
		if err != nil {
			return nil, err
		}

		projectMembers = append(projectMembers, pm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	page.ProjectMembers = projectMembers
	page.Order = cursor.Order

	page.PageCount = uint(page.TotalCount / uint64(cursor.Limit))
	if page.TotalCount%uint64(cursor.Limit) != 0 {
		page.PageCount++
	}

	page.CurrentPage = cursor.Page

	return page, err
}

// GetPagedWithInvitationsByProjectID is a method for querying project members and invitations from the database by projectID, offset and limit.
func (pm *projectMembers) GetPagedWithInvitationsByProjectID(ctx context.Context, projectID uuid.UUID, cursor console.ProjectMembersCursor) (_ *console.ProjectMembersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	search := "%" + strings.ReplaceAll(cursor.Search, " ", "%") + "%"

	if cursor.Limit > 50 {
		cursor.Limit = 50
	}

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

	countRow := pm.db.QueryRowContext(ctx,
		countQuery,
		projectID[:],
		search)

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

	membersQuery := `
		SELECT member_id, project_id, created_at, email, inviter_id FROM (
			(
				SELECT pm.member_id, pm.project_id, pm.created_at, u.email, u.full_name, NULL as inviter_id
				FROM project_members pm
				INNER JOIN users u ON pm.member_id = u.id
				WHERE pm.project_id = $1
				AND (
					u.email ILIKE $2 OR
					u.full_name ILIKE $2 OR
					u.short_name ILIKE $2
				)
			) UNION ALL (
				SELECT NULL as member_id, project_id, created_at, LOWER(email) as email, LOWER(SPLIT_PART(email, '@', 1)) as full_name, inviter_id
				FROM project_invitations pi
				WHERE project_id = $1
				AND email ILIKE $2
			)
		) results
		` + projectMembersSortClause(cursor.Order, page.OrderDirection) + `
		LIMIT $3 OFFSET $4`

	rows, err := pm.db.QueryContext(ctx,
		membersQuery,
		projectID[:],
		search,
		page.Limit,
		page.Offset,
	)
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
			createdAt time.Time
			email     string
			inviterID uuid.NullUUID
		)

		err = rows.Scan(
			&memberID,
			&projectID,
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
func (pm *projectMembers) Insert(ctx context.Context, memberID, projectID uuid.UUID) (_ *console.ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	createdProjectMember, err := pm.methods.Create_ProjectMember(ctx,
		dbx.ProjectMember_MemberId(memberID[:]),
		dbx.ProjectMember_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return projectMemberFromDBX(ctx, createdProjectMember)
}

// Delete is a method for deleting project member by memberID and projectID from the database.
func (pm *projectMembers) Delete(ctx context.Context, memberID, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = pm.methods.Delete_ProjectMember_By_MemberId_And_ProjectId(
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
		CreatedAt: projectMember.CreatedAt,
	}, nil
}

// sanitizedOrderColumnName return valid order by column.
func sanitizedOrderColumnName(pmo console.ProjectMemberOrder) string {
	switch pmo {
	case 2:
		return "u.email"
	case 3:
		return "pm.created_at"
	default:
		return "u.full_name"
	}
}

func sanitizeOrderDirectionName(pmo console.OrderDirection) string {
	if pmo == 2 {
		return "DESC"
	}

	return "ASC"
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
	rs, errors := convertSliceWithErrors(projectMembersDbx,
		func(v *dbx.ProjectMember) (r console.ProjectMember, _ error) {
			p, err := projectMemberFromDBX(ctx, v)
			if err != nil {
				return r, err
			}
			return *p, err
		})
	return rs, errs.Combine(errors...)
}

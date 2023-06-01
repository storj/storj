// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// Ensure that projectInvitations implements console.ProjectInvitations.
var _ console.ProjectInvitations = (*projectInvitations)(nil)

// projectInvitations is an implementation of console.ProjectInvitations.
type projectInvitations struct {
	db *satelliteDB
}

// Insert inserts a project member invitation into the database.
func (invites *projectInvitations) Insert(ctx context.Context, invite *console.ProjectInvitation) (_ *console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	if invite == nil {
		return nil, Error.New("invitation is nil")
	}

	createFields := dbx.ProjectInvitation_Create_Fields{}
	if invite.InviterID != nil {
		id := invite.InviterID[:]
		createFields.InviterId = dbx.ProjectInvitation_InviterId(id)
	}

	dbxInvite, err := invites.db.Create_ProjectInvitation(ctx,
		dbx.ProjectInvitation_ProjectId(invite.ProjectID[:]),
		dbx.ProjectInvitation_Email(normalizeEmail(invite.Email)),
		createFields,
	)
	if err != nil {
		return nil, err
	}

	return projectInvitationFromDBX(dbxInvite)
}

// Get returns a project member invitation from the database.
func (invites *projectInvitations) Get(ctx context.Context, projectID uuid.UUID, email string) (_ *console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInvite, err := invites.db.Get_ProjectInvitation_By_ProjectId_And_Email(ctx,
		dbx.ProjectInvitation_ProjectId(projectID[:]),
		dbx.ProjectInvitation_Email(normalizeEmail(email)),
	)
	if err != nil {
		return nil, err
	}

	return projectInvitationFromDBX(dbxInvite)
}

// GetByProjectID returns all of the project member invitations for the project specified by the given ID.
func (invites *projectInvitations) GetByProjectID(ctx context.Context, projectID uuid.UUID) (_ []console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInvites, err := invites.db.All_ProjectInvitation_By_ProjectId(ctx, dbx.ProjectInvitation_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return projectInvitationSliceFromDBX(dbxInvites)
}

// GetByEmail returns all of the project member invitations for the specified email address.
func (invites *projectInvitations) GetByEmail(ctx context.Context, email string) (_ []console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInvites, err := invites.db.All_ProjectInvitation_By_Email(ctx, dbx.ProjectInvitation_Email(normalizeEmail(email)))
	if err != nil {
		return nil, err
	}

	return projectInvitationSliceFromDBX(dbxInvites)
}

// Delete removes a project member invitation from the database.
func (invites *projectInvitations) Delete(ctx context.Context, projectID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = invites.db.Delete_ProjectInvitation_By_ProjectId_And_Email(ctx,
		dbx.ProjectInvitation_ProjectId(projectID[:]),
		dbx.ProjectInvitation_Email(normalizeEmail(email)),
	)
	return err
}

// DeleteBefore deletes project member invitations created prior to some time from the database.
func (invites *projectInvitations) DeleteBefore(
	ctx context.Context, before time.Time, asOfSystemTimeInterval time.Duration, pageSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if pageSize <= 0 {
		return Error.New("expected page size to be positive; got %d", pageSize)
	}

	var pageCursor, pageEnd struct {
		ProjectID uuid.UUID
		Email     string
	}
	aost := invites.db.impl.AsOfSystemInterval(asOfSystemTimeInterval)
	for {
		// Select the ID beginning this page of records
		err := invites.db.QueryRowContext(ctx, `
			SELECT project_id, email FROM project_invitations
			`+aost+`
			WHERE (project_id, email) > ($1, $2) AND created_at < $3
			ORDER BY (project_id, email) LIMIT 1
		`, pageCursor.ProjectID, pageCursor.Email, before).Scan(&pageCursor.ProjectID, &pageCursor.Email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return Error.Wrap(err)
		}

		// Select the ID ending this page of records
		err = invites.db.QueryRowContext(ctx, `
			SELECT project_id, email FROM project_invitations
			`+aost+`
			WHERE (project_id, email) > ($1, $2)
			ORDER BY (project_id, email) LIMIT 1 OFFSET $3
		`, pageCursor.ProjectID, pageCursor.Email, pageSize).Scan(&pageEnd.ProjectID, &pageEnd.Email)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return Error.Wrap(err)
			}
			// Since this is the last page, we want to return all remaining records
			_, err = invites.db.ExecContext(ctx, `
				DELETE FROM project_invitations
				WHERE (project_id, email) IN (
					SELECT project_id, email FROM project_invitations
					`+aost+`
					WHERE (project_id, email) >= ($1, $2)
					AND created_at < $3
					ORDER BY (project_id, email)
				)
			`, pageCursor.ProjectID, pageCursor.Email, before)
			return Error.Wrap(err)
		}

		// Delete all old, unverified records in the range between the beginning and ending IDs
		_, err = invites.db.ExecContext(ctx, `
			DELETE FROM project_invitations
			WHERE (project_id, email) IN (
				SELECT project_id, email FROM project_invitations
				`+aost+`
				WHERE (project_id, email) >= ($1, $2)
				AND (project_id, email) <= ($3, $4)
				AND created_at < $5
				ORDER BY (project_id, email)
			)
		`, pageCursor.ProjectID, pageCursor.Email, pageEnd.ProjectID, pageEnd.Email, before)
		if err != nil {
			return Error.Wrap(err)
		}

		// Advance the cursor to the next page
		pageCursor = pageEnd
	}
}

// projectInvitationFromDBX converts a project member invitation from the database to a *console.ProjectInvitation.
func projectInvitationFromDBX(dbxInvite *dbx.ProjectInvitation) (_ *console.ProjectInvitation, err error) {
	if dbxInvite == nil {
		return nil, Error.New("dbx invitation is nil")
	}

	invite := &console.ProjectInvitation{
		Email:     dbxInvite.Email,
		CreatedAt: dbxInvite.CreatedAt,
	}

	projectID, err := uuid.FromBytes(dbxInvite.ProjectId)
	if err != nil {
		return nil, err
	}
	invite.ProjectID = projectID

	if dbxInvite.InviterId != nil {
		inviterID, err := uuid.FromBytes(dbxInvite.InviterId)
		if err != nil {
			return nil, err
		}
		invite.InviterID = &inviterID
	}

	return invite, nil
}

// projectInvitationSliceFromDBX converts a project member invitation slice from the database to a
// slice of console.ProjectInvitation.
func projectInvitationSliceFromDBX(dbxInvites []*dbx.ProjectInvitation) (invites []console.ProjectInvitation, err error) {
	return convertSlice(dbxInvites,
		func(i *dbx.ProjectInvitation) (console.ProjectInvitation, error) {
			r, err := projectInvitationFromDBX(i)
			return *r, err
		})
}

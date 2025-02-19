// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/private/slices2"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// Ensure that projectInvitations implements console.ProjectInvitations.
var _ console.ProjectInvitations = (*projectInvitations)(nil)

// projectInvitations is an implementation of console.ProjectInvitations.
type projectInvitations struct {
	db dbx.Methods
}

// Upsert updates a project member invitation if it exists and inserts it otherwise.
func (invites *projectInvitations) Upsert(ctx context.Context, invite *console.ProjectInvitation) (_ *console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	if invite == nil {
		return nil, Error.New("invitation is nil")
	}

	createFields := dbx.ProjectInvitation_Create_Fields{}
	if invite.InviterID != nil {
		id := invite.InviterID[:]
		createFields.InviterId = dbx.ProjectInvitation_InviterId(id)
	}

	dbxInvite, err := invites.db.Replace_ProjectInvitation(ctx,
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

// GetByEmail returns all the project member invitations for the specified email address.
func (invites *projectInvitations) GetByEmail(ctx context.Context, email string) (_ []console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInvites, err := invites.db.All_ProjectInvitation_By_Email(ctx, dbx.ProjectInvitation_Email(normalizeEmail(email)))
	if err != nil {
		return nil, err
	}

	return projectInvitationSliceFromDBX(dbxInvites)
}

// GetForActiveProjectsByEmail returns all project member invitations associated with active projects for the specified email address.
func (invites *projectInvitations) GetForActiveProjectsByEmail(ctx context.Context, email string) (_ []console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInvites, err := invites.db.All_ProjectInvitation_By_Project_Status_And_ProjectInvitation_Email(ctx, dbx.Project_Status(int(console.ProjectActive)), dbx.ProjectInvitation_Email(normalizeEmail(email)))
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
	return slices2.Convert(dbxInvites,
		func(i *dbx.ProjectInvitation) (console.ProjectInvitation, error) {
			r, err := projectInvitationFromDBX(i)
			return *r, err
		})
}

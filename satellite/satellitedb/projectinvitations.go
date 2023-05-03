// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// Ensure that projectInvitations implements console.ProjectInvitations.
var _ console.ProjectInvitations = (*projectInvitations)(nil)

// projectInvitations is an implementation of console.ProjectInvitations.
type projectInvitations struct {
	db dbx.Methods
}

// Insert is a method for inserting a project member invitation into the database.
func (invites *projectInvitations) Insert(ctx context.Context, projectID uuid.UUID, email string) (_ *console.ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInvite, err := invites.db.Create_ProjectInvitation(ctx,
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

// Delete is a method for deleting a project member invitation from the database.
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

	projectID, err := uuid.FromBytes(dbxInvite.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.ProjectInvitation{
		ProjectID: projectID,
		Email:     dbxInvite.Email,
		CreatedAt: dbxInvite.CreatedAt,
	}, nil
}

// projectInvitationSliceFromDBX converts a project member invitation slice from the database to a
// slice of console.ProjectInvitation.
func projectInvitationSliceFromDBX(dbxInvites []*dbx.ProjectInvitation) (invites []console.ProjectInvitation, err error) {
	for _, dbxInvite := range dbxInvites {
		invite, err := projectInvitationFromDBX(dbxInvite)
		if err != nil {
			return nil, err
		}
		invites = append(invites, *invite)
	}
	return invites, nil
}

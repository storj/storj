// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/multinodedb/dbx"
)

// MembersDBError indicates about internal MembersDB error.
var MembersDBError = errs.Class("MembersDB error")

// ensures that members implements console.Members.
var _ console.Members = (*members)(nil)

// members exposes needed by MND MembersDB functionality.
// dbx implementation of console.Members.
//
// architecture: Database
type members struct {
	methods dbx.Methods
}

// Invite will create empty row in membersDB.
func (m *members) Invite(ctx context.Context, member console.Member) (err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := uuid.New()
	if err != nil {
		return MembersDBError.Wrap(err)
	}

	_, err = m.methods.Create_Member(ctx, dbx.Member_Id(id[:]), dbx.Member_Email(member.Email), dbx.Member_Name(member.Name), dbx.Member_PasswordHash(member.PasswordHash))

	return MembersDBError.Wrap(err)
}

// Update updates all updatable fields of member.
func (m *members) Update(ctx context.Context, member console.Member) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = m.methods.Update_Member_By_Id(ctx, dbx.Member_Id(member.ID[:]), dbx.Member_Update_Fields{
		Email:        dbx.Member_Email(member.Email),
		Name:         dbx.Member_Name(member.Name),
		PasswordHash: dbx.Member_PasswordHash(member.PasswordHash),
	})

	return MembersDBError.Wrap(err)
}

// Remove deletes member from membersDB.
func (m *members) Remove(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = m.methods.Delete_Member_By_Id(ctx, dbx.Member_Id(id[:]))

	return MembersDBError.Wrap(err)
}

// GetByEmail will return member with specified email.
func (m *members) GetByEmail(ctx context.Context, email string) (_ console.Member, err error) {
	defer mon.Task()(&ctx)(&err)

	memberDbx, err := m.methods.Get_Member_By_Email(ctx, dbx.Member_Email(email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return console.Member{}, console.ErrNoMember.Wrap(err)
		}
		return console.Member{}, MembersDBError.Wrap(err)
	}

	member, err := fromDBXMember(memberDbx)

	return member, MembersDBError.Wrap(err)
}

// GetByID will return member with specified id.
func (m *members) GetByID(ctx context.Context, id uuid.UUID) (_ console.Member, err error) {
	defer mon.Task()(&ctx)(&err)

	memberDbx, err := m.methods.Get_Member_By_Id(ctx, dbx.Member_Id(id[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return console.Member{}, console.ErrNoMember.Wrap(err)
		}
		return console.Member{}, MembersDBError.Wrap(err)
	}

	member, err := fromDBXMember(memberDbx)

	return member, MembersDBError.Wrap(err)
}

// fromDBXMember converts dbx.Member to console.Member.
func fromDBXMember(member *dbx.Member) (_ console.Member, err error) {
	id, err := uuid.FromBytes(member.Id)
	if err != nil {
		return console.Member{}, err
	}

	result := console.Member{
		ID:           id,
		Email:        member.Email,
		Name:         member.Name,
		PasswordHash: member.PasswordHash,
	}

	return result, nil
}

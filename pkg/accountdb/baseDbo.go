// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountdb

import (
	"github.com/google/uuid"
	"time"
)

// Base entity that has all common fields
type BaseDbo interface {
	Id() uuid.UUID
	CreationDate() time.Time
}

type baseDbo struct {
	id uuid.UUID
	creationDate time.Time
}

func (d *baseDbo) Id() uuid.UUID {
	return d.id
}

func (d *baseDbo) CreationDate() time.Time {
	return d.creationDate
}

func NewBaseDbo(id uuid.UUID, creationDate time.Time) *baseDbo {
	return &baseDbo{
		id,
		creationDate,
	}
}
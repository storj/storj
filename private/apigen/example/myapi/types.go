// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package myapi

import (
	"time"

	"storj.io/common/uuid"
)

// Document is a retrieved document.
type Document struct {
	ID        uuid.UUID `json:"id"`
	Date      time.Time `json:"date"`
	PathParam string    `json:"pathParam"`
	Body      string    `json:"body"`
	Version   uint      `json:"version"`
}

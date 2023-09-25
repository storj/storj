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
	Version   Version   `json:"version"`
}

// Version is document version.
type Version struct {
	Date   time.Time `json:"date"`
	Number uint      `json:"number"`
}

// Metadata is metadata associated to a document.
type Metadata struct {
	Owner string      `json:"owner"`
	Tags  [][2]string `json:"tags"`
}

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
	Metadata  Metadata  `json:"metadata"`
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

// NewDocument contains the content the data to create a new document.
type NewDocument struct {
	Content string `json:"content"`
}

// User contains information of a user.
type User struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
}

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
	Owner string      `json:"owner,omitempty"`
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
	Professional
}

// Professional contains the company and the position where a person works.
type Professional struct {
	Company  string `json:"company"`
	Position string `json:"position"`
}

// UserAge represents a user's age.
//
// The value is generic for being able to increase the year's size to afford to recompile the code
// when we have users that were born in a too far DC year or allow to register users in a few years
// in the future. JOKING, we need it for testing that the API generator works fine with them.
type UserAge[T ~int16 | int32 | int64] struct {
	Day   uint8 `json:"day"`
	Month uint8 `json:"month"`
	Year  T     `json:"year"`
}

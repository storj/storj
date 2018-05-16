// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"math/rand"
	"time"
)

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// String -- Get random string of specified length
func String(length int) string {
	return stringWithCharset(length, charset)
}

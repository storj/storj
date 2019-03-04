// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storjql

import (
	"sync"
)

// mu allows to lock graphql methods, because some of them are not thread-safe
var mu sync.Mutex

// WithLock locks graphql methods, because some of them are not thread-safe
func WithLock(fn func()) {
	mu.Lock()
	defer mu.Unlock()

	fn()
}

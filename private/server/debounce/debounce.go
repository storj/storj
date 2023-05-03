// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package debounce

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/zeebo/blake3"
	"github.com/zyedidia/generic/list"
)

var (
	// ErrDuplicateMessage is returned when a message has been duplicated.
	ErrDuplicateMessage = errors.New("duplicate initial message")
)

var (
	// timeNow is overridable for testing.
	timeNow = time.Now
)

type messageHash [32]byte

type entry struct {
	messageHash messageHash
	firstSeen   time.Time
	count       int
}

// Debouncer makes sure messages with the same hash are not repeated.
type Debouncer struct {
	mtx      sync.Mutex
	maxAge   time.Duration
	maxCount int
	entries  list.List[entry]
	lookup   map[messageHash]*list.Node[entry]
}

// NewDebouncer makes a Debouncer. Messages will only be stored in memory
// up until maxAge time, and once the same message has been received
// maxCount times it will be forgotten as well.
// maxCount is ignored when <= 0.
func NewDebouncer(maxAge time.Duration, maxCount int) *Debouncer {
	return &Debouncer{
		maxAge:   maxAge,
		maxCount: maxCount,
		entries:  list.List[entry]{},
		lookup:   map[messageHash]*list.Node[entry]{},
	}
}

// ResponderFirstMessageValidator is for use in noiseconn.Options.
func (d *Debouncer) ResponderFirstMessageValidator(addr net.Addr, message []byte) error {
	hash := blake3.Sum256(message)
	now := timeNow()
	d.mtx.Lock()
	defer d.mtx.Unlock()

	d.gc(now)

	if n, found := d.lookup[hash]; found {
		n.Value.count++
		if n.Value.count >= d.maxCount && d.maxCount > 0 {
			delete(d.lookup, n.Value.messageHash)
			d.entries.Remove(n)
		}
		return fmt.Errorf("%w: from %s", ErrDuplicateMessage, addr.String())
	}

	n := &list.Node[entry]{
		Value: entry{
			messageHash: hash,
			firstSeen:   now,
			count:       1,
		},
	}
	d.lookup[hash] = n
	d.entries.PushFrontNode(n)
	return nil
}

func (d *Debouncer) gc(now time.Time) {
	for {
		n := d.entries.Back
		if n == nil {
			break
		}
		if now.Sub(n.Value.firstSeen) <= d.maxAge {
			break
		}
		delete(d.lookup, n.Value.messageHash)
		d.entries.Remove(n)
	}
}

// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventkitspy

import (
	"strconv"
	"testing"

	"storj.io/eventkit"
)

func TestDestination(t *testing.T) {
	const limit = 8
	destination := NewDestination(limit)

	test := func(count int) {
		if count > limit {
			panic("test does not support setting count higher than limit")
		}
		destination.read = destination.write
		for i := range count {
			destination.Submit(&eventkit.Event{Name: strconv.Itoa(i)})
		}
		for i, ev := range destination.GetEvents() {
			if ev.Name != strconv.Itoa(i) {
				t.Errorf("expected event name %s, got %s", strconv.Itoa(i), ev.Name)
			}
		}
	}

	test(1)
	test(4)
	test(3)
	test(8)
	test(7)
}

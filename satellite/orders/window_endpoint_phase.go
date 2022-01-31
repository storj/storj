// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"fmt"
	"strings"

	"github.com/zeebo/errs"
)

// WindowEndpointRolloutPhase controls the phase of the new orders endpoint rollout.
type WindowEndpointRolloutPhase int

const (
	// WindowEndpointRolloutPhase1 is when both the old and new endpoint are enabled and
	// the new endpoint places orders in the queue just like the old endpoint.
	WindowEndpointRolloutPhase1 WindowEndpointRolloutPhase = 1 + iota

	// WindowEndpointRolloutPhase2 is when the old endpoint is disabled and the new endpint
	// places orders in the queue just like the old endpoint used to.
	WindowEndpointRolloutPhase2

	// WindowEndpointRolloutPhase3 is when the old endpoint is disabled and the new endpoint
	// does not use a queue and just does direct insertion of rollup values.
	WindowEndpointRolloutPhase3
)

// String provides a human readable form of the rollout phase.
func (phase WindowEndpointRolloutPhase) String() string {
	switch phase {
	case WindowEndpointRolloutPhase1:
		return "phase1"
	case WindowEndpointRolloutPhase2:
		return "phase2"
	case WindowEndpointRolloutPhase3:
		return "phase3"
	default:
		return fmt.Sprintf("WindowEndpointRolloutPhase(%d)", int(phase))
	}
}

// Set implements flag.Value interface.
func (phase *WindowEndpointRolloutPhase) Set(s string) error {
	switch strings.ToLower(s) {
	case "phase1":
		*phase = WindowEndpointRolloutPhase1
	case "phase2":
		*phase = WindowEndpointRolloutPhase2
	case "phase3":
		*phase = WindowEndpointRolloutPhase3
	default:
		return errs.New("invalid window endpoint rollout phase: %q", s)
	}
	return nil
}

// Type implements pflag.Value.
func (WindowEndpointRolloutPhase) Type() string { return "orders.WindowEndpointRolloutPhase" }

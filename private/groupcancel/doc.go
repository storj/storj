// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package groupcancel contains helpers to cancel groups of requests when enough finish.
//
// The main type is the Context type which takes a total number of requests, the fraction
// of the non-failed requests that must succeed before the rest are canceled, and an
// extra wait fraction to multiply by the time it took to succeed that will be waited
// before the rest are canceled.
package groupcancel

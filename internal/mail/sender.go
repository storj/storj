// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import "net/smtp"

// Sender encapsulates information about the mail sender
type Sender struct {
	Mail string
	Auth smtp.Auth
}

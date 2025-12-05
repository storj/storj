// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package post

import (
	"net/mail"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessage_ClosingLastPart(t *testing.T) {
	from := mail.Address{Name: "No reply", Address: "noreply@eu1.storj.io"}

	m := &Message{
		From:      from,
		To:        []mail.Address{{Name: "Foo Bar", Address: "foo@storj.io"}},
		Subject:   "This is a proper test mail",
		PlainText: "",
		Parts: []Part{
			{
				Type:    "text/html; charset=UTF-8",
				Content: string("<head><body><h1>ahoj</h1></body></head>"),
			},
		},
	}

	data, err := m.Bytes()
	require.NoError(t, err)
	lines := strings.Split(string(data), "\n")

	// last part should be closed. see 7.2.1 of https://www.w3.org/Protocols/rfc1341/7_2_Multipart.html
	final := regexp.MustCompile("--.+--")
	lastNonEmptyLine := lines[len(lines)-2]
	require.True(t, final.MatchString(lastNonEmptyLine), "Last line '%s' doesn't include RFC1341 distinguished delimiter", lastNonEmptyLine)
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package mail

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
	"time"
)

// Message is RFC compliant email message
type Message struct {
	From      Address
	To        []Address
	Subject   string
	ID        string
	Date      time.Time
	ReceiptTo []string

	ContentType string
	Encoding    string
	Disposition string

	Body  string
	Parts []Part
}

// Address is email address of a sender/recipient with name
type Address struct {
	Email string
	Name  string
}

// Part represent one part of multipart message
type Part struct {
	Type        string
	Encoding    string
	Disposition string
	Content     string
}

// Bytes builds message and returns result as bytes
func (msg *Message) Bytes() ([]byte, error) {
	// always returns nil on read and write, so most of the errors can be ignored
	var body bytes.Buffer

	// write headers
	fmt.Fprintf(&body, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&body, "Subject: %v\r\n", msg.Subject)
	fmt.Fprintf(&body, "From: %s\r\n", formatAddress(msg.From))
	for _, to := range msg.To {
		fmt.Fprintf(&body, "To: %s\r\n", formatAddress(to))
	}
	for _, recipient := range msg.ReceiptTo {
		fmt.Fprintf(&body, "Disposition-Notification-To: <%v>\r\n", recipient)
	}
	// date and id are optional as they can be set by server itself
	if !msg.Date.IsZero() {
		fmt.Fprintf(&body, "Date: %v\r\n", msg.Date)
	}
	if msg.ID != "" {
		fmt.Fprintf(&body, "Message-ID: <%v>\r\n", msg.ID)
	}

	// parse content type
	mtype, _, err := mime.ParseMediaType(msg.ContentType)
	if err != nil {
		return nil, err
	}

	switch {
	// multipart upload, body is treated as plain text content of the message
	// to support readability
	case strings.HasPrefix(mtype, "multipart"):
		wr := multipart.NewWriter(&body)

		fmt.Fprintf(&body, "Content-Type: %s;", msg.ContentType)
		fmt.Fprintf(&body, "\tboundary=\"%v\"\r\n", wr.Boundary())
		fmt.Fprintf(&body, "\r\n")

		var sub io.Writer

		if len(msg.Body) > 0 {
			sub, _ = wr.CreatePart(textproto.MIMEHeader{
				"Content-Type":              []string{"text/plain; charset=UTF-8; format=flowed"},
				"Content-Transfer-Encoding": []string{"quoted-printable"},
			})

			enc := quotedprintable.NewWriter(sub)
			_, _ = enc.Write([]byte(msg.Body))
			_ = enc.Close()
		}

		for _, part := range msg.Parts {
			header := textproto.MIMEHeader{"Content-Type": []string{part.Type}}
			if part.Encoding != "" {
				header["Content-Transfer-Encoding"] = []string{part.Encoding}
			}
			if part.Disposition != "" {
				header["Content-Disposition"] = []string{part.Disposition}
			}

			sub, _ = wr.CreatePart(header)
			fmt.Fprint(sub, part.Content)
		}

		_ = wr.Close()
		// single part content stored in body, parts are ignored
	default:
		fmt.Fprintf(&body, "Content-Type: %s;", msg.ContentType)
		if msg.Encoding != "" {
			fmt.Fprintf(&body, "Content-Transfer-Encoding: %s;", msg.Encoding)
		}
		if msg.Disposition != "" {
			fmt.Fprintf(&body, "Content-Disposition: %s;", msg.Disposition)
		}
		fmt.Fprintf(&body, "\r\n")
		fmt.Fprintf(&body, msg.Body)
	}

	return tocrlf(body.Bytes()), nil
}

func formatAddress(address Address) string {
	if address.Name != "" {
		return fmt.Sprintf("%s <%v>", address.Name, address.Email)
	}
	return fmt.Sprintf("<%v>", address.Email)
}

func tocrlf(data []byte) []byte {
	lf := bytes.Replace(data, []byte("\r\n"), []byte("\n"), -1)
	crlf := bytes.Replace(lf, []byte("\n"), []byte("\r\n"), -1)
	return crlf
}

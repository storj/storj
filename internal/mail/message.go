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
	"net/mail"
	"net/textproto"
	"time"
)

// Message is RFC compliant email message
type Message struct {
	From      mail.Address
	To        []mail.Address
	Subject   string
	ID        string
	Date      time.Time
	ReceiptTo []string

	PlainText string
	Parts     []Part
}

// Part represent one part of multipart message
type Part struct {
	Type        string
	Encoding    string
	Disposition string
	Content     string
}

// Bytes builds message and returns result as bytes
func (msg *Message) Bytes() []byte {
	// always returns nil error on read and write, so most of the errors can be ignored
	var body bytes.Buffer

	// write headers
	fmt.Fprintf(&body, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&body, "Subject: %v\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&body, "From: %s\r\n", &msg.From)
	for _, to := range msg.To {
		fmt.Fprintf(&body, "To: %s\r\n", &to)
	}
	for _, recipient := range msg.ReceiptTo {
		fmt.Fprintf(&body, "Disposition-Notification-To: <%v>\r\n", mime.QEncoding.Encode("utf-8", recipient))
	}
	// date and id are optional as they can be set by server itself
	if !msg.Date.IsZero() {
		fmt.Fprintf(&body, "Date: %v\r\n", msg.Date)
	}
	if msg.ID != "" {
		fmt.Fprintf(&body, "Message-ID: <%v>\r\n", mime.QEncoding.Encode("utf-8", msg.ID))
	}

	switch {
	// multipart upload
	case len(msg.Parts) > 0:
		wr := multipart.NewWriter(&body)

		fmt.Fprintf(&body, "Content-Type: multipart/alternative;")
		fmt.Fprintf(&body, "\tboundary=\"%v\"\r\n", wr.Boundary())
		fmt.Fprintf(&body, "\r\n")

		var sub io.Writer

		if len(msg.PlainText) > 0 {
			sub, _ = wr.CreatePart(textproto.MIMEHeader{
				"Content-Type":              []string{"text/plain; charset=UTF-8; format=flowed"},
				"Content-Transfer-Encoding": []string{"quoted-printable"},
			})

			enc := quotedprintable.NewWriter(sub)
			_, _ = enc.Write([]byte(msg.PlainText))
			_ = enc.Close()
		}

		for _, part := range msg.Parts {
			header := textproto.MIMEHeader{"Content-Type": []string{mime.QEncoding.Encode("utf-8", part.Type)}}
			if part.Encoding != "" {
				header["Content-Transfer-Encoding"] = []string{mime.QEncoding.Encode("utf-8", part.Encoding)}
			}
			if part.Disposition != "" {
				header["Content-Disposition"] = []string{mime.QEncoding.Encode("utf-8", part.Disposition)}
			}

			sub, _ = wr.CreatePart(header)
			fmt.Fprint(sub, part.Content)
		}

		_ = wr.Close()
	// fallback if there are no parts, write PlainText with appropriate Content-Type
	default:
		fmt.Fprintf(&body, "Content-Type: text/plain; charset=UTF-8; format=flowed\r\n")
		fmt.Fprintf(&body, "Content-Transfer-Encoding: quoted-printable\r\n\r\n")

		enc := quotedprintable.NewWriter(&body)
		_, _ = enc.Write([]byte(msg.PlainText))
		_ = enc.Close()
	}

	return tocrlf(body.Bytes())
}

func tocrlf(data []byte) []byte {
	lf := bytes.Replace(data, []byte("\r\n"), []byte("\n"), -1)
	crlf := bytes.Replace(lf, []byte("\n"), []byte("\r\n"), -1)
	return crlf
}

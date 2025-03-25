// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package simulate

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/html"

	"storj.io/storj/private/post"
	"storj.io/storj/satellite/mailservice"
)

var mon = monkit.Package()

var _ mailservice.Sender = (*LinkClicker)(nil)

// LinkClicker is mailservice.Sender that click all links from html msg parts.
//
// architecture: Service
type LinkClicker struct {
	log *zap.Logger

	// MarkerAttribute specifies the attribute every anchor element must have in order to be clicked.
	// This prevents the link clicker from clicking links that it should not (such as the password reset cancellation link).
	// Leaving this field empty will make it click every link.
	markerAttribute string
}

// NewDefaultLinkClicker returns a LinkClicker with the default marker attribute.
func NewDefaultLinkClicker(log *zap.Logger) *LinkClicker {
	return &LinkClicker{
		log:             log,
		markerAttribute: "data-simulate",
	}
}

// FromAddress return empty mail address.
func (clicker *LinkClicker) FromAddress() post.Address {
	return post.Address{}
}

// SendEmail click all links belonging to properly attributed anchors from email html parts.
func (clicker *LinkClicker) SendEmail(ctx context.Context, msg *post.Message) (err error) {
	defer mon.Task()(&ctx)(&err)

	var body string
	for _, part := range msg.Parts {
		body += part.Content
	}

	// click all links
	var sendError error
	for _, link := range clicker.FindLinks(body) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			continue
		}
		clicker.log.Debug("clicking", zap.String("url", link))
		client := &http.Client{}
		client.Timeout = 5 * time.Second
		response, err := client.Do(req)
		if err != nil {
			clicker.log.Error("failed to click", zap.String("url", link), zap.Error(err))
			continue
		}
		sendError = errs.Combine(sendError, err, response.Body.Close())
	}

	return sendError
}

// FindLinks returns a list of all links belonging to properly attributed anchors in the HTML body.
func (clicker *LinkClicker) FindLinks(body string) (links []string) {
	tokens := html.NewTokenizer(strings.NewReader(body))
Loop:
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			break Loop
		case html.StartTagToken:
			token := tokens.Token()
			if strings.ToLower(token.Data) == "a" {
				simulate := clicker.markerAttribute == ""
				var href string
				for _, attr := range token.Attr {
					if strings.ToLower(attr.Key) == "href" {
						href = attr.Val
					} else if !simulate && attr.Key == clicker.markerAttribute {
						simulate = true
					}
				}
				if simulate && href != "" {
					links = append(links, href)
				}
			}
		}
	}
	return links
}

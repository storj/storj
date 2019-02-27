package simulate

import (
	"bytes"
	"net/http"

	"github.com/zeebo/errs"
	"golang.org/x/net/html"

	"storj.io/storj/internal/post"
)

// LinkClicker is mailservice.Sender that click all links
// from html msg parts
type LinkClicker struct{}

// FromAddress return empty mail address
func (clicker *LinkClicker) FromAddress() post.Address {
	return post.Address{}
}

// SendEmail click all links from email html parts
func (clicker *LinkClicker) SendEmail(msg *post.Message) error {
	// collect all links
	var links []string
	for _, part := range msg.Parts {
		if part.Type != "text/html; charset=UTF-8" {
			continue
		}

		buffer := bytes.NewBufferString(msg.Parts[0].Content)
		tokenizer := html.NewTokenizer(buffer)

	Loop:
		for {
			tokenType := tokenizer.Next()

			switch tokenType {
			case html.ErrorToken:
				break Loop
			case html.StartTagToken:
				token := tokenizer.Token()
				if token.Data == "a" {
					for _, attr := range token.Attr {
						if attr.Key == "href" {
							links = append(links, attr.Val)
						}
					}
				}
			default:
				continue
			}
		}
	}

	// click all links
	var sendError error
	for _, link := range links {
		_, err := http.Get(link)
		sendError = errs.Combine(sendError, err)
	}

	return sendError
}

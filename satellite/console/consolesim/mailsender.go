package consolesim

import (
	"net/http"
	"strings"

	"storj.io/storj/satellite/console/consoleweb/consoleql"

	"storj.io/storj/internal/post"
	"storj.io/storj/satellite/console"
)

// MailSender is smtp sender mock that ignores sending process
type MailSender struct {
	DB console.DB
}

// FromAddress return empty mail address
func (sender *MailSender) FromAddress() post.Address {
	return post.Address{}
}

// SendEmail tries to activate account from email
func (sender *MailSender) SendEmail(msg *post.Message) error {
	if msg.Subject != consoleql.ActivationSubject {
		return nil
	}

	content := msg.Parts[0].Content
	index := strings.Index(content, "Verify your account")

	linkEndOffset := 0
	linkStartOffset := 0

	for i := index; i >= 0; i-- {
		if content[i] == byte('"') {
			linkEndOffset = i
			break
		}
	}

	for i := linkEndOffset - 1; i >= 0; i-- {
		if content[i] == byte('"') {
			linkStartOffset = i + 1
			break
		}
	}

	link := content[linkStartOffset:linkEndOffset]
	_, err := http.Get(link)
	return err
}

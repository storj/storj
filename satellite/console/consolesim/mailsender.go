package consolesim

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/internal/post"
	"storj.io/storj/satellite/console"
)

// MailsSender is smtp sender mock that ignores sending process
type MailsSender struct {
	DB console.DB
}

// FromAddress return empty mail address
func (sender *MailsSender) FromAddress() post.Address {
	return post.Address{}
}

// SendEmail tries to activate account from email
func (sender *MailsSender) SendEmail(msg *post.Message) error {
	if msg.Subject != console.ActivationSubject {
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

	token := content[linkStartOffset:linkEndOffset]
	tokenParts := strings.Split(token, ".")

	jsonb, err := base64.StdEncoding.DecodeString(tokenParts[0])
	if err != nil {
		return err
	}

	var claims struct {
		ID string
	}

	err = json.NewDecoder(bytes.NewReader(jsonb)).Decode(&claims)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(claims.ID)
	if err != nil {
		return err
	}

	ctx := context.Background()
	userStore := sender.DB.Users()

	user, err := userStore.Get(ctx, *id)
	if err != nil {
		return err
	}

	user.Status = console.Active
	return userStore.Update(ctx, user)
}

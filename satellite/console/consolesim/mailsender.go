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
	//token := content[linkStartOffset+len("activationToken=") - 2:linkEndOffset]
	//tokenParts := strings.Split(token, ".")
	//
	//jsonb, err := base64.StdEncoding.DecodeString(tokenParts[0])
	//if err != nil {
	//	return err
	//}
	//
	//var claims struct {
	//	ID string
	//}
	//
	//err = json.NewDecoder(bytes.NewReader(jsonb)).Decode(&claims)
	//if err != nil {
	//	return err
	//}
	//
	//id, err := uuid.Parse(claims.ID)
	//if err != nil {
	//	return err
	//}
	//
	//ctx := context.Background()
	//userStore := sender.DB.Users()
	//
	//user, err := userStore.Get(ctx, *id)
	//if err != nil {
	//	return err
	//}
	//
	//user.Status = console.Active
	//return userStore.Update(ctx, user)
}

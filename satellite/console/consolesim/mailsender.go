package consolesim

import "storj.io/storj/internal/post"

// MailsSender is smtp sender mock that ignores sending process
type MailsSender struct {
}

// FromAddress return empty mail address
func (sender *MailsSender) FromAddress() post.Address {
	return post.Address{}
}

// SendEmail ignores actual mail sending and return nil
func (sender *MailsSender) SendEmail(msg *post.Message) error {
	// TODO(yar): may be it should write to a file or smth
	return nil
}

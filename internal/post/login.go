package post

import (
	"net/smtp"

	"github.com/zeebo/errs"
)

// LoginAuth implements LOGIN authentication mechanism
type LoginAuth struct {
	Username string
	Password string
}

// Start begins an authentication with a server
func (auth LoginAuth) Start(server *smtp.ServerInfo) (proto string, toServer []byte, err error) {
	if !server.TLS {
		return "", nil, errs.New("unencrypted connection")
	}
	return "LOGIN", nil, nil
}

// Next continues the authentication with server response and flag representing
// if server expects more data from client
func (auth LoginAuth) Next(fromServer []byte, more bool) (toServer []byte, err error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(auth.Username), nil
		case "Password:":
			return []byte(auth.Password), nil
		default:
			return nil, errs.New("unknown question")
		}
	}
	return nil, nil
}

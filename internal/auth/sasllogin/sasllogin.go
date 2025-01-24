package sasllogin

import "github.com/emersion/go-sasl"

// Copy-pasted from old emersion/go-sasl version

// Authenticates users with an username and a password.
type LoginAuthenticator func(username, password string) error
type loginState int

const (
	loginNotStarted loginState = iota
	loginWaitingUsername
	loginWaitingPassword
)

type loginServer struct {
	state              loginState
	username, password string
	authenticate       LoginAuthenticator
}

// A server implementation of the LOGIN authentication mechanism, as described
// in https://tools.ietf.org/html/draft-murchison-sasl-login-00.
//
// LOGIN is obsolete and should only be enabled for legacy clients that cannot
// be updated to use PLAIN.
func NewLoginServer(authenticator LoginAuthenticator) sasl.Server {
	return &loginServer{authenticate: authenticator}
}

func (a *loginServer) Next(response []byte) (challenge []byte, done bool, err error) {
	switch a.state {
	case loginNotStarted:
		// Check for initial response field, as per RFC4422 section 3
		if response == nil {
			challenge = []byte("Username:")
			break
		}
		a.state++
		fallthrough
	case loginWaitingUsername:
		a.username = string(response)
		challenge = []byte("Password:")
	case loginWaitingPassword:
		a.password = string(response)
		err = a.authenticate(a.username, a.password)
		done = true
	default:
		err = sasl.ErrUnexpectedClientResponse
	}
	a.state++
	return
}

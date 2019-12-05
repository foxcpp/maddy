package module

// AuthProvider is the interface implemented by modules providing authentication using
// username:password pairs.
type AuthProvider interface {
	CheckPlain(username, password string) bool
}

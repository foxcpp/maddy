package module

// Tabele is the interface implemented by module that implementation string-to-string
// translation.
type Table interface {
	Lookup(s string) (string, bool, error)
}

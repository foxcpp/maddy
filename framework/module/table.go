package module

// Tabele is the interface implemented by module that implementation string-to-string
// translation.
type Table interface {
	Lookup(s string) (string, bool, error)
}

type MutableTable interface {
	Table
	Keys() ([]string, error)
	RemoveKey(k string) error
	SetKey(k, v string) error
}

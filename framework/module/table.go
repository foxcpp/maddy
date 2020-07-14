package module

// Tabele is the interface implemented by module that implementation string-to-string
// translation.
//
// Modules implementing this interface should be registered with prefix
// "table." in name.
type Table interface {
	Lookup(s string) (string, bool, error)
}

type MutableTable interface {
	Table
	Keys() ([]string, error)
	RemoveKey(k string) error
	SetKey(k, v string) error
}

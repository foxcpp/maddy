package testutils

type Table struct {
	M   map[string]string
	Err error
}

func (m Table) Lookup(a string) (string, bool, error) {
	b, ok := m.M[a]
	return b, ok, m.Err
}

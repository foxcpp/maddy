package table

import (
	"context"

	"github.com/foxcpp/maddy/framework/module"
	lru "github.com/hashicorp/golang-lru/v2"
)

type CachedTable struct {
	source module.Table
	cache  *lru.TwoQueueCache[string, string]
}

var _ module.Table = CachedTable{}

func NewCachedTable(source module.Table, size int) (module.Table, error) {
	cache, err := lru.New2Q[string, string](size)
	if err != nil {
		return nil, err
	}

	return CachedTable{
		source: source,
		cache:  cache,
	}, nil
}

func (t CachedTable) Lookup(ctx context.Context, key string) (string, bool, error) {
	value, ok := t.cache.Get(key)
	if ok {
		return value, true, nil
	}

	value, ok, err := t.source.Lookup(ctx, key)
	if err != nil {
		return "", ok, err
	}

	t.cache.Add(key, value)

	return value, ok, nil
}

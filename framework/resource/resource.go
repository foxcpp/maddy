package resource

import (
	"io"
)

type Resource = io.Closer

type CheckableResource interface {
	Resource
	IsUsable() bool
}

type Container[T Resource] interface {
	io.Closer
	GetOpen(key string, open func() (T, error)) (T, error)
	CloseUnused(isUsed func(key string) bool) error
}

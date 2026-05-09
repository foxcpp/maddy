package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotSupported = errors.New("storage operation not supported")

type AccountDTO struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type AccountOptions struct{}

type FolderDTO struct {
	ID   string
	Path string
	Role string
}

type FolderOptions struct {
	Role string
}

type MessageDTO struct {
	ID         string
	ReceivedAt time.Time
	Title      string
	Flags      []string
}

// ManagedStorage is a minimalist storage interface for CLI access.
type ManagedStorage interface {
	ListAccounts(ctx context.Context, substring string, limit, offset int) ([]AccountDTO, error)
	CreateAccount(ctx context.Context, name string, options AccountOptions) (AccountDTO, error)
	DeleteAccount(ctx context.Context, id string) error

	ListFolders(ctx context.Context, accountName string) ([]FolderDTO, error)
	CreateFolder(ctx context.Context, accountName, folderName string, options FolderOptions) (FolderDTO, error)
	RenameFolder(ctx context.Context, accountName, oldPath string, newPath string) error
	DeleteFolder(ctx context.Context, accountName, path string) error

	ListMessages(ctx context.Context, accountName, folderPath string, limit, offset int) ([]MessageDTO, error)
	AddMessage(ctx context.Context, accountName, folderPath string, date time.Time, r io.Reader) (MessageDTO, error)
	CopyMessages(ctx context.Context, accountName, folderPath, toFolderPath string, ids []string) ([]MessageDTO, error)
	MoveMessages(ctx context.Context, accountName, folderPath, toFolderPath string, ids []string) ([]MessageDTO, error)
	DeleteMessages(ctx context.Context, accountName, folderPath string, ids []string) error
	RemoveFlag(ctx context.Context, accountName, folderPath string, ids []string, flag string) error
	AddFlag(ctx context.Context, accountName, folderPath string, ids []string, flag string) error
	DumpMessage(ctx context.Context, accountName, folderPath string, id string) (io.ReadCloser, error)
}

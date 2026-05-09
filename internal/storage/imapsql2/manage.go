package imapsql2

import (
	"context"
	"io"
	"time"

	"github.com/foxcpp/maddy-storage/pkg/managed"
	"github.com/foxcpp/maddy/framework/module/storage"
)

func (store *Storage) ListAccounts(ctx context.Context, substring string, limit, offset int) ([]storage.AccountDTO, error) {
	accounts, err := store.store.Manager().ListAccounts(ctx, substring, limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]storage.AccountDTO, len(accounts))
	for i, acct := range accounts {
		result[i] = storage.AccountDTO(acct)
	}
	return result, nil
}

func (store *Storage) CreateAccount(ctx context.Context, name string, options storage.AccountOptions) (storage.AccountDTO, error) {
	acct, err := store.store.Manager().CreateAccount(ctx, name, managed.AccountOptions(options))
	if err != nil {
		return storage.AccountDTO{}, err
	}
	return storage.AccountDTO(acct), nil
}

func (store *Storage) DeleteAccount(ctx context.Context, id string) error {
	return store.store.Manager().DeleteAccount(ctx, id)
}

func (store *Storage) ListFolders(ctx context.Context, accountName string) ([]storage.FolderDTO, error) {
	folders, err := store.store.Manager().ListFolders(ctx, accountName)
	if err != nil {
		return nil, err
	}

	result := make([]storage.FolderDTO, len(folders))
	for i, folder := range folders {
		result[i] = storage.FolderDTO(folder)
	}
	return result, nil
}

func (store *Storage) CreateFolder(ctx context.Context, accountName, folderName string, options storage.FolderOptions) (storage.FolderDTO, error) {
	folder, err := store.store.Manager().CreateFolder(ctx, accountName, folderName, managed.FolderOptions(options))
	if err != nil {
		return storage.FolderDTO{}, err
	}
	return storage.FolderDTO(folder), nil
}

func (store *Storage) RenameFolder(ctx context.Context, accountName, oldPath string, newPath string) error {
	return store.store.Manager().RenameFolder(ctx, accountName, oldPath, newPath)
}

func (store *Storage) DeleteFolder(ctx context.Context, accountName, folderPath string) error {
	return store.store.Manager().DeleteFolder(ctx, accountName, folderPath)
}

func (store *Storage) ListMessages(ctx context.Context, accountName, folderPath string, limit, offset int) ([]storage.MessageDTO, error) {
	messages, err := store.store.Manager().ListMessages(ctx, accountName, folderPath, limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]storage.MessageDTO, len(messages))
	for i, msg := range messages {
		result[i] = storage.MessageDTO(msg)
	}
	return result, nil
}

func (store *Storage) AddMessage(ctx context.Context, accountName, folderPath string, date time.Time, r io.Reader) (storage.MessageDTO, error) {
	msg, err := store.store.Manager().AddMessage(ctx, accountName, folderPath, date, r)
	if err != nil {
		return storage.MessageDTO{}, err
	}
	return storage.MessageDTO(msg), nil
}

func (store *Storage) CopyMessages(ctx context.Context, accountName, folderPath, toFolderPath string, ids []string) ([]storage.MessageDTO, error) {
	messages, err := store.store.Manager().CopyMessages(ctx, accountName, folderPath, toFolderPath, ids)
	if err != nil {
		return nil, err
	}

	result := make([]storage.MessageDTO, len(messages))
	for i, msg := range messages {
		result[i] = storage.MessageDTO(msg)
	}
	return result, nil
}

func (store *Storage) MoveMessages(ctx context.Context, accountName, folderPath, toFolderPath string, ids []string) ([]storage.MessageDTO, error) {
	messages, err := store.store.Manager().MoveMessages(ctx, accountName, folderPath, toFolderPath, ids)
	if err != nil {
		return nil, err
	}

	result := make([]storage.MessageDTO, len(messages))
	for i, msg := range messages {
		result[i] = storage.MessageDTO(msg)
	}
	return result, nil
}

func (store *Storage) DeleteMessages(ctx context.Context, accountName, folderPath string, ids []string) error {
	return store.store.Manager().DeleteMessages(ctx, accountName, folderPath, ids)
}

func (store *Storage) RemoveFlag(ctx context.Context, accountName, folderPath string, ids []string, flag string) error {
	return store.store.Manager().RemoveFlag(ctx, accountName, folderPath, ids, flag)
}

func (store *Storage) AddFlag(ctx context.Context, accountName, folderPath string, ids []string, flag string) error {
	return store.store.Manager().AddFlag(ctx, accountName, folderPath, ids, flag)
}

func (store *Storage) DumpMessage(ctx context.Context, accountName, folderPath string, id string) (io.ReadCloser, error) {
	return store.store.Manager().DumpMessage(ctx, accountName, folderPath, id)
}

package dbcache

import (
	"context"
	"errors"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type UserStorage struct {
	cache *lru.Cache[int64, *model.UserIdentity]
	next  state.UserStorage
}

var _ state.UserStorage = &UserStorage{}

func NewUserStorage(size int, nx state.UserStorage) *UserStorage {
	cache, err := lru.New[int64, *model.UserIdentity](size)
	if err != nil {
		panic(err)
	}
	return &UserStorage{
		cache: cache,
		next:  nx,
	}
}

func (s *UserStorage) FetchUsers(ctx context.Context) ([]*model.UserIdentity, error) {
	return s.next.FetchUsers(ctx)
}

func (s *UserStorage) CreateUser(ctx context.Context, nick string, emailAddress string, passwordHash string, isAdmin bool) error {
	return s.next.CreateUser(ctx, nick, emailAddress, passwordHash, isAdmin)
}

func (s *UserStorage) FetchUserByUserID(ctx context.Context, id int64) (*model.UserIdentity, error) {
	if ui, ok := s.cache.Get(id); ok {
		return ui.Clone(), nil
	}

	ui, error := s.next.FetchUserByUserID(ctx, id)
	if error == nil {
		s.cache.Add(id, ui.Clone())
	}

	return ui, error
}

func (s *UserStorage) FetchUserRow(ctx context.Context, nick string) (*model.UserRow, error) {
	return s.next.FetchUserRow(ctx, nick)
}

func (s *UserStorage) DeleteUserByNick(ctx context.Context, nick string) error {
	return s.next.DeleteUserByNick(ctx, nick)
}

func (s *UserStorage) AddPassword(ctx context.Context, userID int64, passwordHash string) error {
	return s.next.AddPassword(ctx, userID, passwordHash)
}

func (s *UserStorage) RemoveExpiredPasswords(ctx context.Context, before time.Time) error {
	return s.next.RemoveExpiredPasswords(ctx, before)
}

func (s *UserStorage) ReplacePassword(ctx context.Context, userID int64, newPasswordHash string, oldPasswordsExpire time.Time) error {
	return s.next.ReplacePassword(ctx, userID, newPasswordHash, oldPasswordsExpire)
}

func unimplemented() error {
	return errors.New("fix me")
}

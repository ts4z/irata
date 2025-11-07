package permission

import (
	"context"
	"errors"
	"time"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
)

type UserStorage struct {
	next state.UserStorage
}

var _ state.UserStorage = &UserStorage{}

func NewUserStorage(nx state.UserStorage) *UserStorage {
	return &UserStorage{
		next: nx,
	}
}

func requireAdminOrUserID(ctx context.Context, uid int64, fn func() error) error {
	if ui := UserFromContext(ctx); ui == nil {
		return errors.New("no user in context")
	} else if !ui.IsAdmin && ui.ID != uid {
		return errors.New("permission denied")
	} else {
		return fn()
	}
}

func requireUserAdminReturning[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	u := UserFromContext(ctx)
	if u == nil || !u.IsAdmin {
		return zero, errors.New("permission denied")
	}
	return fn()
}

func requireUserAdmin(ctx context.Context, fn func() error) error {
	_, err := requireUserAdminReturning(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func (s *UserStorage) FetchUsers(ctx context.Context) ([]*model.UserIdentity, error) {
	return requireUserAdminReturning(ctx, func() ([]*model.UserIdentity, error) {
		return s.next.FetchUsers(ctx)
	})
}

func (s *UserStorage) CreateUser(ctx context.Context, nick string, emailAddress string, passwordHash string, isAdmin bool) error {
	return requireUserAdmin(ctx, func() error {
		return s.next.CreateUser(ctx, nick, emailAddress, passwordHash, isAdmin)
	})
}

func (s *UserStorage) FetchUserByUserID(ctx context.Context, id int64) (*model.UserIdentity, error) {
	return s.next.FetchUserByUserID(ctx, id)
}

// TODO this requires a non-user context, as this is the hook that enables uer login.
func (s *UserStorage) FetchUserRow(ctx context.Context, nick string) (*model.UserRow, error) {
	return s.next.FetchUserRow(ctx, nick)
}

func (s *UserStorage) DeleteUserByNick(ctx context.Context, nick string) error {
	return requireUserAdmin(ctx, func() error {
		return s.next.DeleteUserByNick(ctx, nick)
	})
}

func (s *UserStorage) AddPassword(ctx context.Context, userID int64, passwordHash string) error {
	return requireUserAdmin(ctx, func() error {
		return s.next.AddPassword(ctx, userID, passwordHash)
	})
}

func (s *UserStorage) RemoveExpiredPasswords(ctx context.Context, before time.Time) error {
	return s.next.RemoveExpiredPasswords(ctx, before)
}

func (s *UserStorage) ReplacePassword(ctx context.Context, userID int64, newPasswordHash string, oldPasswordsExpire time.Time) error {
	return requireAdminOrUserID(ctx, userID, func() error {
		return s.next.ReplacePassword(ctx, userID, newPasswordHash, oldPasswordsExpire)
	})
}

package dbcache

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/state"
	"github.com/ts4z/irata/varz"
)

var (
	userStorageCacheHits   = varz.NewInt("userStorageCacheHits")
	userStorageCacheMisses = varz.NewInt("userStoragecacheMisses")
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

// TODO: We need to be able to call this for multiple-writer changes.
//
// TODO: We need to consume the version here; this will invalidate it too often.
// However, the UserIdentity field doesn't have a Version yet.
func (s *UserStorage) CacheInvalidate(_ context.Context, userID int64, version int64) {
	if version < 0 {
		s.cache.Remove(userID)
	} else {
		_, ok := s.cache.Get(userID)
		if ok {
			// TODO: Add Version to UserIdentity and check it.
			s.cache.Remove(userID)
		}
	}
}

func (s *UserStorage) Fetch(ctx context.Context, id int64) (*model.UserIdentity, error) {
	return s.FetchUserByUserID(ctx, id)
}

func (s *UserStorage) FetchUsers(ctx context.Context) ([]*model.UserIdentity, error) {
	return s.next.FetchUsers(ctx)
}

func (s *UserStorage) CreateUser(ctx context.Context, u *model.UserIdentity) (int64, error) {
	return s.next.CreateUser(ctx, u)
}

func (s *UserStorage) CreateUserWithEmailAndPassword(ctx context.Context, nick string, emailAddress string, passwordHash string, isAdmin bool) error {
	return s.next.CreateUserWithEmailAndPassword(ctx, nick, emailAddress, passwordHash, isAdmin)
}

func (s *UserStorage) FetchUserByUserID(ctx context.Context, id int64) (*model.UserIdentity, error) {
	if ui, ok := s.cache.Get(id); ok {
		userStorageCacheHits.Add(1)
		return ui.Clone(), nil
	}

	userStorageCacheMisses.Add(1)

	ui, error := s.next.FetchUserByUserID(ctx, id)
	if error == nil {
		s.cache.Add(id, ui.Clone())
	}

	return ui, error
}

func (s *UserStorage) FetchUserRow(ctx context.Context, nick string) (*model.UserRow, error) {
	return s.next.FetchUserRow(ctx, nick)
}

func (s *UserStorage) SaveUser(ctx context.Context, u *model.UserIdentity) error {
	err := s.next.SaveUser(ctx, u)
	if err == nil {
		s.cache.Add(u.ID, u.Clone())
	}
	return err
}

func (s *UserStorage) DeleteUserByID(ctx context.Context, id int64) error {
	err := s.next.DeleteUserByID(ctx, id)
	if err == nil {
		// TODO: Remove, this should get handled by gossip.
		s.CacheInvalidate(ctx, id, -1)
	}
	return err
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

package gossip

import (
	"context"

	"github.com/ts4z/irata/dbnotify"
	"github.com/ts4z/irata/model"
)

type UserGossiper struct {
	cacheStorage CacheStorage[model.UserIdentity]
}

// NotifyUpdated implements dbnotify.ClientNotifier.
func (u *UserGossiper) NotifyUpdated(ctx context.Context, m *model.UserIdentity) {
	// dropped, no way to subscribe to these.
}

var _ dbnotify.ClientNotifier[*model.UserIdentity] = &UserGossiper{}

func NewUserGossiper(cache CacheStorage[model.UserIdentity]) *UserGossiper {
	return &UserGossiper{
		cacheStorage: cache,
	}
}

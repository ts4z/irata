package c2ctx

import (
	"context"
	"log"
	"net/http"

	"github.com/ts4z/irata/dep"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/state"
)

// CookieToContext is middleware that parses the cookie from the request and squirrels
// it away in the context, so not every level of the app has to be aware of users.
type CookieToContext struct {
	bakeryFactory *permission.BakeryFactory
	userStorage   state.UserStorage

	next http.Handler
}

func (c *CookieToContext) fetchUserFromCookie(ctx context.Context, r *http.Request) (*model.UserIdentity, error) {
	bakery, err := c.bakeryFactory.Bakery(ctx)
	if err != nil {
		return nil, err
	}
	cookieData, err := bakery.ReadCookie(r)
	if err != nil {
		return nil, err
	}

	identity, err := c.userStorage.FetchUserByUserID(ctx, cookieData.EffectiveUserID)
	if err != nil {
		log.Printf("can't fetch user %+v: %v", cookieData.EffectiveUserID, err)
	}
	return identity, nil
}

// ServeHTTP implements the http.Handler interface and forwards to the next handler
func (c *CookieToContext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	identity, err := c.fetchUserFromCookie(ctx, r)
	if err != nil {
		// Probably doesn't need to log.
		log.Printf("can't fetch user data from cookie: %v", err)
	} else {
		ctx = permission.UserIdentityInContext(ctx, identity)
		r = r.WithContext(ctx)
	}

	c.next.ServeHTTP(w, r)
}

type Config struct {
	BakeryFactory *permission.BakeryFactory
	UserStorage   state.UserStorage
	Next          http.Handler
}

func Handler(cf *Config) http.Handler {
	return &CookieToContext{
		bakeryFactory: dep.Required(cf.BakeryFactory),
		userStorage:   dep.Required(cf.UserStorage),
		next:          dep.Required(cf.Next),
	}
}

package c2ctx

import (
	"context"
	"log"
	"net/http"

	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/permission"
	"github.com/ts4z/irata/state"
)

// CookieToContext is middleware that parses the cookie from the request and squirrels
// it away in the context, so not every level of the app has to be aware of users.
type CookieToContext struct {
	bakery      *permission.Bakery
	userStorage state.UserStorage

	next http.Handler
}

func (c *CookieToContext) fetchUserFromCookie(ctx context.Context, r *http.Request) (*model.UserIdentity, error) {
	cookieData, err := c.bakery.ReadCookie(r)
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
	Bakery      *permission.Bakery
	UserStorage state.UserStorage
	Next        http.Handler
}

func Handler(cf *Config) http.Handler {
	return &CookieToContext{
		bakery:      cf.Bakery,
		userStorage: cf.UserStorage,
		next:        cf.Next,
	}
}

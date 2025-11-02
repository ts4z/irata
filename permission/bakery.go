package permission

/*
Package permission knows who you are and what you're allowed to do.

TODO: Cookies aren't automatically rotated.  Site config is set at server
boot time, which is wrong, but not wrong very often.
*/

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"

	"github.com/ts4z/irata/config"
	"github.com/ts4z/irata/model"
)

const (
	AuthCookieName = "irata-auth"
)

type BakeryClock interface {
	Now() time.Time
}

type cookieBaker struct {
	cookieDomain string
	v            model.CookieKeyValidity
	sc           *securecookie.SecureCookie
}

func (cb *cookieBaker) honorable(now time.Time) bool {
	return now.After(cb.v.MintFrom) && now.Before(cb.v.MintUntil)
}

func (cb *cookieBaker) mintable(now time.Time) bool {
	return now.After(cb.v.MintFrom) && now.Before(cb.v.MintUntil)
}

type SiteConfigFetcher interface {
	FetchSiteConfig(ctx context.Context) (*model.SiteConfig, error)
}

type Bakery struct {
	cookieDomain string
	bakers       []cookieBaker
}

// New creates a new Bakery instance.
func New(clock BakeryClock, conf *model.SiteConfig) (*Bakery, error) {
	now := clock.Now()
	keys := []cookieBaker{}
	for i, inputKey := range conf.CookieKeys {
		if inputKey.Validity.HonorUntil.Before(now) {
			log.Printf("disregarding key conf.CookieKeys[%d] since it is expired", i)
			continue
		}
		hashKey, err := base64.StdEncoding.DecodeString(inputKey.HashKey64)
		if err != nil {
			log.Printf("disregarding key conf.CookieKeys[%d] due to bad HashKey64: %v", i, err)
			continue
		}
		blockKey, err := base64.StdEncoding.DecodeString(inputKey.BlockKey64)
		if err != nil {
			log.Printf("disregarding key conf.CookieKeys[%d] due to bad BlockKey64: %v", i, err)
			continue
		}
		keys = append(keys,
			cookieBaker{
				cookieDomain: conf.CookieDomain,
				sc:           securecookie.New(hashKey, blockKey),
				v:            inputKey.Validity,
			})
	}

	log.Printf("bakery: %d valid keys", len(keys))

	return &Bakery{
		bakers: keys,
	}, nil
}

func (b *Bakery) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    AuthCookieName,
		Value:   "",
		Expires: time.Unix(-1, 0),
	})
}

func (b *Bakery) ReadCookie(r *http.Request) (*model.AuthCookieData, error) {
	cookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		return nil, fmt.Errorf("can't get cookie: %w", err)
	}

	errors := []error{}

	c := &model.AuthCookieData{}
	for _, baker := range b.bakers {
		if !baker.honorable(time.Now()) {
			continue
		}

		err := baker.sc.Decode(AuthCookieName, cookie.Value, c)
		if err == nil {
			return c, nil
		}

		errors = append(errors, err)
	}

	if len(errors) == 0 {
		return nil, fmt.Errorf("no valid keys to validate cookie")
	}
	return nil, fmt.Errorf("can't validate cookie (%d decoders): %w", len(errors), errors[0])
}

func (b *Bakery) bestKeyForMinting(now time.Time) (*cookieBaker, error) {
	var best *cookieBaker
	for _, key := range b.bakers {
		if !key.mintable(now) {
			continue
		}

		// Pick the key that is valid for the longest amount of time.
		if best == nil || best.v.HonorUntil.Before(key.v.HonorUntil) {
			best = &key
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no valid key for minting")
	}

	return best, nil
}

func (b *Bakery) BakeCookie(w http.ResponseWriter, lc *model.AuthCookieData) error {
	bb, err := b.bestKeyForMinting(time.Now())
	if err != nil {
		return fmt.Errorf("can't find key for minting: %w", err)
	}

	encrypted, err := bb.sc.Encode(AuthCookieName, lc)
	if err != nil {
		log.Printf("can't encrypt cookie: %v", err)
		return fmt.Errorf("can't encrypt cookie: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AuthCookieName,
		Value:    encrypted,
		Path:     "/",
		Domain:   b.cookieDomain,
		Secure:   config.SecureCookies(),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	return nil
}

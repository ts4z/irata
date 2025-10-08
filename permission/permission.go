package permission

import (
	"context"
	"net/http"

	"github.com/ts4z/irata/he"
)

type EmailAddress string

func (em EmailAddress) String() string {
	return string(em)
}

// private guarantees that Auth structs were made by this package.
type private struct{}

type Identity struct {
	emailAddress EmailAddress
}

// User combines both auth-n and auth-z in one confused package.
type User struct {
	private
	effective *Identity
	isAdmin   bool
}

func (a *User) IsAdmin() bool {
	return a.isAdmin
}

func (a *User) Effective() *Identity {
	return a.effective
}

type contextKeyType struct{}

var contextKeyTypeValue = contextKeyType{}

func InContext(ctx context.Context, a *User) context.Context {
	return context.WithValue(ctx, contextKeyType{}, a)
}

func UserFromContext(ctx context.Context) *User {
	v := ctx.Value(contextKeyTypeValue)
	if a, ok := v.(*User); ok {
		return a
	} else {
		panic("at the disco")
	}
}

// DecoratedContext provides a Context that has a setting for this module,
// so any layer of the stack can recover authz/authn information.
func DecoratedContext(r *http.Request) (context.Context, *User) {
	ctx := r.Context()
	u := &User{
		private:   private{},
		effective: &Identity{emailAddress: "devnull@psaux.com"}, // TODO: implement users
		isAdmin:   true,
	}
	return context.WithValue(ctx, contextKeyTypeValue, u), u
}

func CheckWriteAccessToTournamentID(ctx context.Context, _ int64) error {
	u := UserFromContext(ctx)
	if !u.IsAdmin() {
		return he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied")
	}
	// TODO more checks needed, eh?
	return nil
}

package permission

import (
	"context"
	"net/http"

	"github.com/ts4z/irata/he"
	"github.com/ts4z/irata/model"
)

type contextKeyType struct{}

var contextKeyTypeValue = contextKeyType{}

func IsAdmin(context context.Context) bool {
	u := UserFromContext(context)
	if u == nil {
		return false
	}
	return u.IsAdmin
}

func IsOperator(context context.Context) bool {
	u := UserFromContext(context)
	if u == nil {
		return false
	}
	return u.IsOperator
}

func UserIdentityInContext(ctx context.Context, a *model.UserIdentity) context.Context {
	return context.WithValue(ctx, contextKeyType{}, a)
}

func UserFromContext(ctx context.Context) *model.UserIdentity {
	v := ctx.Value(contextKeyTypeValue)
	if a, ok := v.(*model.UserIdentity); ok {
		return a
	} else {
		return nil
	}
}

// Deprecated. Replace with requireOperator.
func CheckWriteAccessToTournamentID(ctx context.Context, _ int64) error {
	if !IsOperator(ctx) {
		return he.HTTPCodedErrorf(http.StatusUnauthorized, "permission denied")
	}
	// TODO more checks needed, eh?
	return nil
}

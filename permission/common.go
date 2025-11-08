package permission

import (
	"context"
	"errors"
)

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

func requireOperatorReturning[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	u := UserFromContext(ctx)
	if u == nil || !u.IsOperator {
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

func requireOperator(ctx context.Context, fn func() error) error {
	_, err := requireOperatorReturning(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func requireSiteAdmin(ctx context.Context, fn func() error) error {
	// todo: we need site admin permission here
	return requireUserAdmin(ctx, fn)
}

func requireSiteAdminReturning[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	// TODO: we need site admin permission here
	return requireUserAdminReturning(ctx, fn)
}

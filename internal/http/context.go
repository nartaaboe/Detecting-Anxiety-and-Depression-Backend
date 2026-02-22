package httpapi

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyAuth      ctxKey = "auth"
)

type AuthInfo struct {
	UserID uuid.UUID
	Roles  []string
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyRequestID).(string)
	return v, ok
}

func withAuth(ctx context.Context, a AuthInfo) context.Context {
	return context.WithValue(ctx, ctxKeyAuth, a)
}

func AuthFromContext(ctx context.Context) (AuthInfo, bool) {
	v, ok := ctx.Value(ctxKeyAuth).(AuthInfo)
	return v, ok
}

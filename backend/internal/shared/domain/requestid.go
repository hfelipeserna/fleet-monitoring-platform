package domain

import "context"

type RequestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey{}, id)
}

func RequestIDFromCtx(ctx context.Context) (string, bool) {
	v := ctx.Value(RequestIDKey{})
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

package requestid

import (
	"context"
	"strings"
)

const Header = "X-Request-ID"

type contextKey struct{}

func With(ctx context.Context, value string) context.Context {
	value = strings.TrimSpace(value)
	if value == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, value)
}

func From(ctx context.Context) string {
	value, _ := ctx.Value(contextKey{}).(string)
	return strings.TrimSpace(value)
}

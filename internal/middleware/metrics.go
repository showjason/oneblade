package middleware

import (
	"context"
)

// Metrics middleware placeholder
func Metrics(next func(context.Context, interface{}) (interface{}, error)) func(context.Context, interface{}) (interface{}, error) {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		// Record metrics here
		return next(ctx, req)
	}
}

package middleware

import (
	"context"
	"log"
)

// Logging middleware placeholder
func Logging(next func(context.Context, interface{}) (interface{}, error)) func(context.Context, interface{}) (interface{}, error) {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		log.Printf("Request: %v", req)
		res, err := next(ctx, req)
		log.Printf("Response: %v, Error: %v", res, err)
		return res, err
	}
}

package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	// Create a mock next function
	called := false
	var receivedReq interface{}
	var receivedCtx context.Context

	next := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		receivedReq = req
		receivedCtx = ctx
		return "response", nil
	}

	// Wrap with metrics middleware
	metricsMiddleware := Metrics(next)

	// Call the middleware
	ctx := context.Background()
	req := "test request"
	res, err := metricsMiddleware(ctx, req)

	// Verify next was called
	require.True(t, called, "next function should be called")
	assert.Equal(t, req, receivedReq)
	assert.Equal(t, ctx, receivedCtx)
	assert.Equal(t, "response", res)
	assert.NoError(t, err)
}

func TestMetrics_WithError(t *testing.T) {
	// Create a mock next function that returns an error
	testError := errors.New("test error")
	next := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, testError
	}

	// Wrap with metrics middleware
	metricsMiddleware := Metrics(next)

	// Call the middleware
	ctx := context.Background()
	req := "test request"
	res, err := metricsMiddleware(ctx, req)

	// Verify error is propagated
	assert.Nil(t, res)
	assert.Error(t, err)
	assert.Equal(t, testError, err)
}

func TestMetrics_ContextPreserved(t *testing.T) {
	// Create a mock next function
	next := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Verify context is preserved
		value := ctx.Value("test-key")
		assert.Equal(t, "test-value", value)
		return "response", nil
	}

	// Wrap with metrics middleware
	metricsMiddleware := Metrics(next)

	// Call the middleware with context containing a value
	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	req := "test request"
	_, err := metricsMiddleware(ctx, req)

	assert.NoError(t, err)
}

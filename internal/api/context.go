package api

import (
	"context"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

type contextKey string

const clientContextKey contextKey = "api_client"

// ClientFromContext extracts ApiClient from context
func ClientFromContext(ctx context.Context) *models.ApiClient {
	client, ok := ctx.Value(clientContextKey).(*models.ApiClient)
	if !ok {
		return nil
	}
	return client
}

// ContextWithClient adds ApiClient to context
func ContextWithClient(ctx context.Context, client *models.ApiClient) context.Context {
	return context.WithValue(ctx, clientContextKey, client)
}

package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
)

func NewServer(
	ctx context.Context,
	client *auth.Client,
) http.Handler {
	mux := http.NewServeMux()
	addRoutes(
		ctx,
		mux,
		client,
	)
	var handler http.Handler = mux
	handler = verifyToken(client, handler)
	return handler
}

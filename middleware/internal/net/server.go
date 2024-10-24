package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
	"github.com/Vertisphere/backend-service/internal/storage"
)

func NewServer(
	ctx context.Context,
	auth *auth.Client,
	store *storage.SQLStorage,
	qbClientKeys []string,

) http.Handler {
	mux := http.NewServeMux()
	addRoutes(
		ctx,
		mux,
		auth,
		store,
		qbClientKeys,
	)
	var handler http.Handler = mux
	handler = verifyToken(auth, handler)
	return handler
}

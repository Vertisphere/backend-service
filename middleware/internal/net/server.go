package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/storage"
)

func NewServer(
	ctx context.Context,
	auth *auth.Client,
	store *storage.SQLStorage,
	firebaseClient *fb.Client,
	quickbooksClient *qb.Client,

) http.Handler {
	mux := http.NewServeMux()
	addRoutes(
		ctx,
		mux,
		auth,
		store,
		firebaseClient,
		quickbooksClient,
	)
	var handler http.Handler = mux
	handler = verifyToken(auth, handler)
	return handler
}

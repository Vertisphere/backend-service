package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/storage"
)

func addRoutes(
	ctx context.Context,
	mux *http.ServeMux,
	auth *auth.Client,
	storage *storage.SQLStorage,
	fbc *fb.Client,
	qbc *qb.Client,

) {
	// Assuming that the following routes are prefixed with /api
	mux.Handle("/", http.NotFoundHandler())

	// We're creating a firebase user for franchiser not franchisee
	// Nothing in schema for "user"
	mux.Handle("POST /register", CreateUser(fbc))
}

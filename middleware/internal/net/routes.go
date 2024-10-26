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
	// FOR DEV ONLY
	// mux.Handle("/", http.NotFoundHandler())

	// We're creating a firebase user for franchiser not franchisee
	mux.Handle("POST /qbLogin", LoginQuickbooks(fbc, qbc, auth, storage))

	mux.Handle("GET /customers", ListCustomers(qbc))

	mux.Handle("/", ShowClaims())
	// mux.Handle("POST /register", CreateUser(fbc))

	// mux.Handle("POST /company", CreateCompany(fbc, qbc, storage))
	// email and password -> get firebase user -> get firebase token -> get quickbooks token -> store in db
	// mux.Handle("POST /login", LoginUser(fbc))
}

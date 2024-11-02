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

	// THE FRANCHISER FRANCHISEE prefixes are not really necessary but keeping them for dev clarity purposes for now

	// We're creating a firebase user for franchiser not franchisee
	mux.Handle("POST /franchiser/qbLogin", LoginQuickbooks(fbc, qbc, auth, storage))
	mux.Handle("GET /franchiser/qbCustomer/{id}", GetQBCustomer(qbc, storage))
	mux.Handle("GET /franchiser/qbCustomers", ListQBCustomers(qbc))

	// We're creating a firebase user for franchisee
	mux.Handle("POST /franchiser/customers", CreateCustomer(fbc, qbc, auth, storage))

	// login for franchisee
	mux.Handle("POST /franchisee/login", LoginCustomer(fbc, qbc, auth, storage))
	mux.Handle("GET /franchisee/qbItems", ListQBItems(qbc))
	// TODO add role management in these handlers
	mux.Handle("POST /franchisee/qbInvoice", CreateQBInvoice(qbc))

	mux.Handle("GET /franchiser/qbInvoice/{id}", GetQBInvoice(qbc))
	mux.Handle("PATCH /franchiser/qbInvoice/{id}", ReviewQBInvoice(qbc))

	// This should really be the same thing since we cane use the claims to determine the role
	mux.Handle("GET /franchiser/qbInvoices", ListQBInvoices(qbc))
	mux.Handle("GET /franchisee/qbInvoices", ListQBInvoicesCustomer(qbc))

	// mux.Handle("/", ShowClaims())
	// mux.Handle("POST /register", CreateUser(fbc))

	// mux.Handle("POST /company", CreateCompany(fbc, qbc, storage))
	// email and password -> get firebase user -> get firebase token -> get quickbooks token -> store in db
	// mux.Handle("POST /login", LoginUser(fbc))
}

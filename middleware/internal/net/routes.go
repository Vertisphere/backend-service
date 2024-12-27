package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/storage"
	"github.com/twilio/twilio-go"
)

func addRoutes(
	ctx context.Context,
	mux *http.ServeMux,
	auth *auth.Client,
	storage *storage.SQLStorage,
	fbc *fb.Client,
	qbc *qb.Client,
	twc *twilio.RestClient,

) {
	// mux.Handle("/", http.NotFoundHandler())

	// THE FRANCHISER FRANCHISEE prefixes are not really necessary but keeping them for dev clarity purposes for now

	// Login endpoints (These are ignored in the middleware)
	mux.Handle("POST /franchiser/qbLogin", LoginQuickbooks(fbc, qbc, auth, storage))
	mux.Handle("POST /franchisee/login", LoginCustomer(fbc, qbc, auth, storage))

	// QBCustomers
	mux.Handle("GET /qbCustomer/{id}", GetQBCustomer(qbc, storage))
	mux.Handle("GET /qbCustomers", ListQBCustomers(qbc, storage))

	// QBInvoices
	mux.Handle("GET /qbInvoice/{id}", GetQBInvoice(qbc))
	mux.Handle("GET /qbInvoices", ListQBInvoices(qbc))
	mux.Handle("POST /qbInvoice", CreateQBInvoice(qbc, twc, auth))
	// When order is pending and franchiser wants to accept/reject OR when "order" is accepted and franchisee wants to mark as ready for pickup - "completed".
	mux.Handle("PATCH /qbInvoice/{id}", UpdateQBInvoice(qbc, twc))
	// When "order" is still pending and franchisee wants to update it
	// mux.Handle("PUT /franchiser/qbInvoice/{id}", UpdateQBInvoice(qbc))

	// We're creating a firebase user for franchisee
	mux.Handle("POST /customer", CreateCustomer(fbc, qbc, auth, storage))

	// login for franchisee
	mux.Handle("GET /qbItems", ListQBItems(qbc))
	// TODO add role management in these handlers

	// This should really be the same thing since we cane use the claims to determine the role

	mux.Handle("GET /qbInvoicePDF/{id}", GetQBInvoicePDF(qbc))

	mux.Handle("GET /", ShowClaims())

}

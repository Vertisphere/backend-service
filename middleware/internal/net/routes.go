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

	mux.Handle("GET /qbInvoicePDF/{id}", GetQBInvoicePDF(qbc))

	// Create a Invoice: DRAFT
	mux.Handle("GET /qbInvoice:create", CreateQBInvoice(qbc))

	// modify a Invoice: DRAFT
	mux.Handle("POST /qbInvoice:modify/{id}", UpdateQBInvoice(qbc))

	// Set QBInvoice to pending (review) FROM DRAFT, REVISION
	mux.Handle("GET /qbInvoice:publish/{id}", PublishQBInvoice(qbc, auth, twc))
	// Set QBInvoice to DRAFT FROM PENDING
	mux.Handle("GET /qbInvoice:unpublish/{id}", UnpublishQBInvoice(qbc, auth, twc))
	// Set QBInvoice to approved (in preparation) FROM PENDING
	mux.Handle("GET /qbInvoice:approve/{id}", ApproveQBInvoice(qbc, auth, twc))
	mux.Handle("GET /qbInvoice:void/{id}", VoidQBInvoice(qbc, auth, twc))

	// Duplicate qbInvoice
	mux.Handle("GET /qbInvoice:duplicate/{id}", DuplicateQBInvoice(qbc))

	// Set QBInvoice to revision (needs change)
	// WE'RE NOT USING REJECTED FOR NOW. WILL USE DRAFT INSTEAD
	// mux.Handle("GET /qbInvoice:reject/{id}", RejectQBInvoice(qbc))
	// Set QBInvoice to complete (ready for pick up)
	mux.Handle("GET /qbInvoice:complete/{id}", CompleteQBInvoice(qbc, auth, twc))

	mux.Handle("GET /qbInvoice/{id}", GetQBInvoice(qbc))

	mux.Handle("GET /qbInvoices", ListQBInvoices(qbc))

	// We're creating a firebase user for franchisee
	mux.Handle("POST /customer", CreateCustomer(fbc, qbc, auth, storage))
	// We're deleting a firebase user for franchisee
	mux.Handle("DELETE /customer", DeleteCustomer(auth, storage))

	// login for franchisee
	mux.Handle("GET /qbItems", ListQBItems(qbc))
	// TODO add role management in these handlers

	// This should really be the same thing since we cane use the claims to determine the role

	// mux.Handle("GET /", ShowClaims())

}

package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
	"github.com/Vertisphere/backend-service/internal/storage"
)

func addRoutes(
	ctx context.Context,
	mux *http.ServeMux,
	auth *auth.Client,
	storage *storage.SQLStorage,
) {
	// Assuming that the following routes are prefixed with /api
	mux.Handle("/", http.NotFoundHandler())
	// Get Custom Token
	// mux.Handle('/customToken', handleCustomToken(auth))

	// Create Franchise with franchiser user
	mux.Handle("POST /franchise", handlePostFranchise(auth, storage))

	// Create user and add to franchise

	// login is handled by firbase frontend
	// mux.Handle("/login", handleLogin(client))
	// mux.Handle("/register", handleRegister(auth))
	// mux.Handle("/customToken", handleCustomToken(auth, storage))

	// For franchiser
	// /franchisee/:id/orders/:id
	// mux.Handle("GET /franchiser/:id", handleFranchiser(client))
	// mux.Handle("GET /franchiser/:id", handleFranchiser(client))
	// mux.Handle("GET /franchiser")

	// For franchiser
	// franchiser has:
	// - franchiser_id
	// - franchiser_name
	// - user (array of user_id)

	// mux.Handle("GET /franchisee/:id")
	// For franchisee
	// franchisee have:
	// - franchisee_id
	// - franchisee_name/Location?
	// - user (array of user_id)?

	// mux.Handle("GET /orders/:id")
	// For orders
	// orders have:
	// - order_id
	// - order_date
	// - order_status
	// - order_items
	// - order_total
	// - order_payment
	// - order_franchisee
	// - order_franchiser Should franchisers just have their own table for all orders?

	// mux.Handle("GET /invoice_")
	// For invoice
	// invoice have:
	// - invoice_id
	// - invoice_date
	// - orders (array of order_id)
	// - invoice_total
	// - invoice-pdf (link to pdf)
}

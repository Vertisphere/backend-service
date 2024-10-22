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
	qbClientKeys []string,
) {
	// Assuming that the following routes are prefixed with /api
	mux.Handle("/", http.NotFoundHandler())
	// Get Custom Token
	mux.Handle("GET /customToken", handleCustomToken(auth, storage))

	// Create Franchise with franchiser user (nvm we're not doing that)
	mux.Handle("POST /franchise", handlePostFranchise(storage))

	mux.Handle("POST /linkQuickbooks", handleLinkQuickbooks(storage, qbClientKeys))
	// Create user (Right now only supports creating franchiser)
	mux.Handle("POST /user", handlePostUser(auth, storage))

	// Create franchisee and sent password reset email
	mux.Handle("POST /franchisee", handlePostFranchisee(auth, storage))

	// Create Product (Only for franchiser)
	mux.Handle("POST /product", handlePostProduct(storage))

	// Search for products
	// List Products
	// mux.Handle("GET /product", handleListProduct(storage))

	// Get product
	// mux.Handle("GET /product/:id", handleGetProduct(storage))

	// Going to follow google REST API guidelines
	// resource: product
	// method: list
	// patterns: pagination
	// Should I make a separate list and search or just make search do everything?
	// For now just make search do everything

	// resource: product
	// method: search
	// patterns: pagination, result ordering
	// fields:
	// 	sorting order:
	// - order_by (https://cloud.google.com/apis/design/design_patterns#sorting_order)
	// What is generally going to be ordered by?
	// price? Created date? ...
	// 	pagination (https://cloud.google.com/apis/design/design_patterns#list_pagination)
	// PS: Pagination with FTS isn't really efficient since you need to get the whole search result and index it
	// To get a page everytime you switch pages.
	// For now we bite the bullet because we expect people wont be going through pages given a search.
	// - page_token
	// - page_size
	// - next_page_token
	// Search:
	// If query is "" then treat call like a list request
	// query:
	// Search for products
	mux.Handle("GET /product:search", handleSearchProduct(storage))

	// Since the serach + list returns description we can store desciption client side for all serach results
	// Although this is isn't that hard to implement
	// mux.Handle("GET /product/:id", handleGetProductInfo(storage))

	// resource: order
	// method: create
	// patterns:
	// fields:
	// Create Order
	mux.Handle("POST /order", handleCreateOrder(storage))

	// Franchisee updates order if still in ordered status
	// mux.Handle("PATCH /order", handleUpdateOrder(storage))

	// Franchiser updates order to preparing status
	// TODO: figure out a smarter way to pass data between middleware -> handler beyond just context
	// We're giving handler quickbooks client through the context like wtf?
	// mux.Handle("PUT /order:approve", isQuickbooks(storage, qbClientKeys, handleApproveOrder(storage)))

	// See all orders for franchise
	// mux.Handle("GET /order", handlePost)

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

package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
)

func addRoutes(
	ctx context.Context,
	mux *http.ServeMux,
	client *auth.Client,
) {
	// login is handled by firbase frontend
	// mux.Handle("/login", handleLogin(client))
	mux.Handle("/register", handleRegister(client))

	mux.Handle("/", http.NotFoundHandler())

	mux.Handle("GET /api", handleSomething())
	mux.Handle("POST /api", handleSomething())
	// mux.Handle("LIST /api", verifyToken(handleSomething()))
	// mux.Handle("PUT /api", verifyToken(handleSomething()))
	// mux.Handle("DELETE /api", verifyToken(handleSomething()))

}

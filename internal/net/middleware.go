package net

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/auth"
)

func verifyToken(c *auth.Client, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// add if condition to check if the request is for the login endpoint
		ctx := r.Context()

		if r.URL.Path == "/login" || r.URL.Path == "/register" {
			r = r.WithContext(context.WithValue(ctx, "client", c))
			h.ServeHTTP(w, r)
			return
		}
		bearer := strings.Split(r.Header.Get("Authorization"), "Bearer ")[1]
		token, err := c.VerifyIDToken(ctx, bearer)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// call handler with token
		r = r.WithContext(context.WithValue(ctx, "token", token))
		h.ServeHTTP(w, r)
	})
}

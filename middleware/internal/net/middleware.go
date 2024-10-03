package net

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/auth"
)

func verifyToken(c *auth.Client, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		authHeader := strings.Split(r.Header.Get("Authorization"), "Bearer ")
		if len(authHeader) != 2 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		bearer := authHeader[1]
		token, err := c.VerifyIDToken(ctx, bearer)
		if err != nil {
			// Tokeen is invalid
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/franchise" && r.Method == "POST" {
			r = r.WithContext(context.WithValue(ctx, "account_id", token.UID))
			h.ServeHTTP(w, r)
			return
		}
		if token.Claims["is_user"] == nil {
			// Token is valid but not custom token
			http.Error(w, "Not Custom Token", http.StatusUnauthorized)
		}
		// call handler with token Claims info (is_franchiser, is_admin)
		r = r.WithContext(context.WithValue(ctx, "token", token.Claims))
		h.ServeHTTP(w, r)
	})
}

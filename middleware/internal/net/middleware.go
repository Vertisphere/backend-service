package net

import (
	"context"
	"log"
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
		if (r.URL.Path == "/customToken" && r.Method == "GET") || ((r.URL.Path == "/franchise" || r.URL.Path == "/user") && r.Method == "POST") {
			r = r.WithContext(context.WithValue(ctx, "account_id", token.UID))
			h.ServeHTTP(w, r)
			return
		}
		log.Println(token.Claims)
		if token.Claims["role"] == nil {
			// Token is valid but not custom token
			http.Error(w, "Not Custom Token", http.StatusUnauthorized)
		}
		// call handler with token Claims info (app_id, franchise_id, role)
		ctx = context.WithValue(ctx, "app_id", token.Claims["app_id"])
		ctx = context.WithValue(ctx, "franchise_id", token.Claims["franchise_id"])
		ctx = context.WithValue(ctx, "role", token.Claims["role"])
		log.Println(token.Claims["franchise_id"])
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

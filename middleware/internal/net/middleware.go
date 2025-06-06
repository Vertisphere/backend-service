package net

import (
	"context"
	"net/http"
	"os"
	"strings"

	"firebase.google.com/go/auth"
	"github.com/Vertisphere/backend-service/internal/domain"
	"github.com/google/uuid"
)

func authMiddleware(c *auth.Client, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/franchiser/qbLogin" && r.Method == "POST" || r.URL.Path == "/franchisee/login" && r.Method == "POST" || r.URL.Path == "/" && r.Method == "GET" {
			h.ServeHTTP(w, r)
			return
		}

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
		if token.Claims["is_franchiser"] == nil {
			http.Error(w, "Not Custom Token", http.StatusUnauthorized)
			return
		}
		claims := domain.Claims{
			IsFranchiser:  token.Claims["is_franchiser"].(bool),
			QBCompanyID:   token.Claims["qb_company_id"].(string),
			QBCustomerID:  token.Claims["qb_customer_id"].(string),
			QBBearerToken: token.Claims["qb_bearer_token"].(string),
			FirebaseID:    token.Claims["firebase_id"].(string),
		}
		ctx = context.WithValue(ctx, "claims", claims)
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: clean this up
		w.Header().Set("Access-Control-Allow-Origin", os.Getenv("CORS_ORIGIN"))
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PATCH, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func traceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate a unique trace ID for this request
		traceID := uuid.New().String()
		// Add trace ID to response headers
		w.Header().Set("X-Trace-ID", traceID)
		r = r.WithContext(context.WithValue(r.Context(), "traceID", traceID))
		next.ServeHTTP(w, r)
	})
}

package net

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"firebase.google.com/go/auth"

	"github.com/Vertisphere/backend-service/internal/quickbooks"
	"github.com/Vertisphere/backend-service/internal/storage"
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
		ctx = context.WithValue(ctx, "app_user_id", token.Claims["app_user_id"])
		ctx = context.WithValue(ctx, "franchise_id", token.Claims["franchise_id"])
		ctx = context.WithValue(ctx, "franchisee_id", token.Claims["franchisee_id"])
		ctx = context.WithValue(ctx, "role", token.Claims["role"])
		// log.Println(token.Claims["franchise_id"])
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

func isQuickbooks(s *storage.SQLStorage, qbClientKeys []string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fuck it, we make a new client everytime we get a request
		franchiseIDValue := r.Context().Value("role")
		franchiseIDFloat, ok := franchiseIDValue.(float64)
		if !ok {
			http.Error(w, "Cannot get role ID from JWT", http.StatusBadRequest)
			return
		}
		franchiseID := int(franchiseIDFloat)

		// Get realmID from DB (we can store this in the )
		// Check in DB if quickbooks auth token and refresh token are valid
		realmId, authToken, jwtToken, exp, err := s.GetQuickbooksAuth(franchiseID)
		if err != nil {
			http.Error(w, "Cannot retrieve quickbooks data from db", http.StatusInternalServerError)
		}

		clientId := qbClientKeys[0]
		clientSecret := qbClientKeys[1]
		clientIsProd := qbClientKeys[2]
		if clientId == "" || clientSecret == "" || clientIsProd == "" {
			http.Error(w, "Quickbooks api client secrets not available", http.StatusInternalServerError)
			return
		}
		// Convert string to bool
		clientIsProdBool, err := strconv.ParseBool(clientIsProd)
		if err != nil {
			http.Error(w, "Invalid value for clientIsProd", http.StatusInternalServerError)
			return
		}

		qbClient, err := quickbooks.NewClient(clientId, clientSecret, realmId, clientIsProdBool, "", nil)
		if err != nil {
			http.Error(w, "Couldn't initialize quickbooks client", http.StatusInternalServerError)
		}

		if authToken == "" {
			http.Error(w, "Franchise not synced to quickbooks", http.StatusBadRequest)

		}

		if jwtToken == "" {
			// Ideally when we make the set the auth code, we should also set the jwt
			// but just in case
			// Use auth token to get jwt
			// This redirectURI doesn't really matter
			redirectURI := "https://developer.intuit.com/v2/OAuth2Playground/RedirectUrl"
			bearerToken, err := qbClient.RetrieveBearerToken(authToken, redirectURI)
			if err != nil {
				http.Error(w, "Cannot retrieve JWT token data from Quickbooks API", http.StatusInternalServerError)
				return
			}
			jwtToken = bearerToken.AccessToken
			if err != nil {
				http.Error(w, "Cannot retrieve JWT token data from Quickbooks API", http.StatusInternalServerError)
				return
			}
		}
		if exp {
			// Use refresh token to get new auth token
			bearerToken, err := qbClient.RefreshToken(jwtToken)
			if err != nil {
				http.Error(w, "Cannot refresh token", http.StatusInternalServerError)
				return
			}
			// set new jwt token in db
			s.SetQuickbooksRefresh(franchiseID, bearerToken.AccessToken)
		}
		// If auth token is expired, then return auth token exp and get new auth token client side
		// If refresh token is expired, then just get new bearer
		// Once we get a valid bearer, initialize client and pass it to the handler so it can call to quickbooks

		// pass to context
		r = r.WithContext(context.WithValue(r.Context(), "qbClient", qbClient))
		h.ServeHTTP(w, r)
	})
}

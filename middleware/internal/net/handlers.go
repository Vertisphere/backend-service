package net

import (
	"fmt"
	"log"
	"net/http"

	"firebase.google.com/go/auth"
	"github.com/Vertisphere/backend-service/internal/domain"
	"github.com/Vertisphere/backend-service/internal/storage"
)

func handlePostFranchise(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	type request struct {
		Franchise domain.Franchise `json:"franchise"`
	}
	type response struct {
		id      string `json:"id"`
		success bool   `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		account_id := r.Context().Value("account_id")
		if account_id == nil {
			// encode(w, r, 401, response{success: false, id: })
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		franchise_id, err := s.CreateFranchise(req.Franchise)
		if err != nil {
			// log error
			log.Println(err)
			http.Error(w, "Could not create franchise", http.StatusInternalServerError)
			return
		}
		// create franchiser user
		// user_id, err := s.CreateFranchiseUser()
		// write to reponse with response struct
		response := response{id: fmt.Sprintf("%d", franchise_id), success: true}
		encode(w, r, 200, response)
	}
}

// func handleCustomToken(c *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		ctx := r.Context()
// 		bearer := strings.Split(r.Header.Get("Authorization"), "Bearer ")[1]
// 		// Should already be verified, but just in case
// 		token, err := c.VerifyIDToken(ctx, bearer)
// 		if err != nil {
// 			// Token is invalid
// 			http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 			return
// 		}
// 		// Get User's Role from DB
// 		id, role, err := s.ReadUserForCustomToken(token.UID)
// 		if err != nil || role == nil {
// 			http.Error(w, "Could not get user role", http.StatusInternalServerError)
// 			return
// 		}

// 		c.CustomTokenWithClaims(ctx, token.UID, map[string]interface{}{
// 			"id":   id,
// 			"role": role,
// 		})
// 		// send custom token back
// 		w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, token)))
// 	}
// }

// func createFranchise(s *storage.SQLStorage) http.HandlerFunc {
// 	type response struct {
// 		id      string `json:"id"`
// 		success bool   `json:"success"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		franchise, err := decode[domain.Franchise](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}
// 		err := s.createFranchise(franchise)
// 		if err != nil {
// 			http.Error(w, "Could not create franchise", http.StatusInternalServerError)
// 			return
// 		}
// 		w.write([]byte(fmt.Sprintf(`{"id": "%s"}`, franchise.Id)))
// 	}
// }

// func createOrder(s *storage.SQLStorage) http.HandlerFunc {
// 	type request struct {
// 		Products []string `json:"products"`
// 	}
// 	type response struct {
// 		OrderID string `json:"order_id"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		var req request
// 		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}
// 		// Assuming s.createOrder is a method that creates an order and returns an order ID
// 		orderID, err := s.CreateOrder(req.Products)
// 		if err != nil {
// 			http.Error(w, "Could not create order", http.StatusInternalServerError)
// 			return
// 		}
// 		resp := response{OrderID: orderID}
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(resp)
// 	}
// }

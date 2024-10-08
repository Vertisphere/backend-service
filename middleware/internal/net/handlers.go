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
		accountIDStr, ok := account_id.(string)
		if !ok {
			http.Error(w, "Invalid account ID", http.StatusBadRequest)
			return
		}

		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		err = s.CreateFranchise(accountIDStr, req.Franchise)
		if err != nil {
			// log error
			log.Println(err)
			http.Error(w, "Could not create franchise", http.StatusInternalServerError)
			return
		}
		// create franchiser user
		// user_id, err := s.CreateFranchiseUser()
		// write to reponse with response struct
		response := response{success: true}
		encode(w, r, 200, response)
	}
}

func handlePostUser(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	// -- 0 for franchisee_non_admin, 1 for franchisee_admin,
	// 2 for franchiser_non_admin, 3 for franchiser_admin
	type request struct {
		Name string `json:"name"`
		Role int    `json:"role"`
	}
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		account_id := r.Context().Value("account_id")
		if account_id == nil {
			// encode(w, r, 401, response{success: false, id: })
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		accountIDStr, ok := account_id.(string)
		if !ok {
			http.Error(w, "Invalid account ID", http.StatusBadRequest)
			return
		}
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		if req.Role == 0 || req.Role == 1 {
			// a.EmailSignInLink(r.Context(), req.Name)
			// s.CreateFranchiseeUser(req.Name, req.Role)
		} else if req.Role == 2 || req.Role == 3 {
			franchiseID, err := s.GetFranchiseIDFromAccountId(accountIDStr)
			if err != nil {
				http.Error(w, "User not designated as franchiser", http.StatusInternalServerError)
				return
			}
			err = s.CreateFranchiseUser(accountIDStr, franchiseID, req.Name)
			if err != nil {
				http.Error(w, "Could not create user, likely already exists", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}
		response := response{Success: true}
		encode(w, r, 200, response)
	}
}

func handleCustomToken(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account_id := r.Context().Value("account_id")
		if account_id == nil {
			// encode(w, r, 401, response{success: false, id: })
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		accountIDStr, ok := account_id.(string)
		if !ok {
			http.Error(w, "Invalid account ID", http.StatusBadRequest)
			return
		}
		// Get User's Role from DB
		app_id, franchise_id, role, err := s.GetUserClaims(accountIDStr)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not get user role", http.StatusInternalServerError)
			return
		}

		token, err := a.CustomTokenWithClaims(r.Context(), accountIDStr, map[string]interface{}{
			"app_id":       app_id,
			"franchise_id": franchise_id,
			"role":         role,
		})
		if err != nil {
			http.Error(w, "Could not create custom token", http.StatusInternalServerError)
			return
		}
		// send custom token back
		w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, token)))
	}
}

func handlePostFranchisee(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	type request struct {
		FranchiseeName   string `json:"franchisee_name"`
		HeadquartersName string `json:"headquarters_address"`
		Email            string `json:"email"`
		Phone            string `json:"phone"`
	}
	type response struct {
		Link    string `json:"link"`
		Success bool   `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		franchiseIDValue := r.Context().Value("franchise_id")
		franchiseIDFloat, ok := franchiseIDValue.(float64)
		if !ok {
			http.Error(w, "Invalid franchise ID", http.StatusBadRequest)
			return
		}
		// Add error handling for float to int conversion
		franchise_id := int(franchiseIDFloat)
		log.Println(franchise_id)
		if franchise_id != 3 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		// Create Anon User in Firebase and get UID
		// Create Franchisee in DB and add UID
		// create app_user in DB and link to franchise, franchisee, and firbase uid
		u := auth.UserToCreate{}
		u.Email(req.Email)
		uid, err := a.CreateUser(r.Context(), &u)
		if err != nil {
			http.Error(w, "Could not create user", http.StatusInternalServerError)
			return
		}
		// TODO: use env variable for URL
		emailSetting := auth.ActionCodeSettings{
			URL: "https://backend-435201.firebaseapp.com",
		}
		a.GetUser(r.Context(), uid.UID)
		link, err := a.PasswordResetLinkWithSettings(r.Context(), req.Email, &emailSetting)

		// TODO: send email with link.
		// Right now admin sdk only let's you create oob link instead of actually sending the email.
		// And the shitty thing is that google cloud run doesn't let you send emails
		// So unless we figure out another hosting, we'll just have to use external email service for now.

		if err != nil {
			http.Error(w, "Could not create email link", http.StatusInternalServerError)
			return
		}

		franchisee_id, err := s.CreateFranchisee(franchise_id, req.FranchiseeName, req.HeadquartersName, req.Phone)
		if err != nil {
			http.Error(w, "Could not create franchisee", http.StatusInternalServerError)
			return
		}
		err = s.CreateFranchiseeUser(uid.UID, franchise_id, franchisee_id, req.FranchiseeName+" Admin")
		if err != nil {
			http.Error(w, "Could not create franchisee", http.StatusInternalServerError)
			return
		}
		response := response{Success: true, Link: link}
		encode(w, r, 200, response)
	}
}

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

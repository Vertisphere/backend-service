package net

import (
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/Vertisphere/backend-service/internal/storage"
	"gopkg.in/square/go-jose.v2"
)

// func CreateUser(fbc *fb.Client) http.HandlerFunc {
// 	type request struct {
// 		Email    string `json:"email"`
// 		Password string `json:"password"`
// 	}

// 	type response struct {
// 		Token string `json:"token"`
// 		Success bool `json:"success"`
// 	}

// 	return func(w http.ResponseWriter, r *http.Request) {
// 		req, err := decode[request](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}
// 		// Create user
// 		err = fbc.CreateUser(req.Email, req.Password)
// 		if err != nil {
// 			http.Error(w, "Could not create user", http.StatusInternalServerError)
// 			return
// 		}
// 		response := response{
// 			Token:
// 			Success: true}
// 		encode(w, r, 200, response)
// 	}
// }

func LoginQuickbooks(fbc *fb.Client, qbc *qb.Client, a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	type request struct {
		AuthCode        string `json:"auth_code"`
		RealmID         string `json:"realm_id"`
		UseCachedBearer bool   `json:"use_cached_bearer"`
	}

	type response struct {
		Name    string `json:"name"`
		Token   string `json:"token"`
		Success bool   `json:"success"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		var bearerToken *qb.BearerToken
		if req.UseCachedBearer {
			dbCompany, err := s.GetCompany(req.RealmID)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not get company from DB", http.StatusInternalServerError)
				return
			}
			if dbCompany.QBBearerToken == "" {
				http.Error(w, "No cached token", http.StatusBadRequest)
				return
			}
			isTokenExpired := dbCompany.QBBearerTokenExpiry.Before(time.Now())
			if isTokenExpired {
				bearerToken, err = qbc.RefreshToken(dbCompany.QBRefreshToken)
				if err != nil {
					log.Println(err)
					http.Error(w, "Could not refresh token in DB", http.StatusInternalServerError)
					return
				}
				err = s.UpsertCompany(dbCompany.QBCompanyID, dbCompany.QBAuthCode, bearerToken.AccessToken, bearerToken.ExpiresIn, bearerToken.RefreshToken, bearerToken.XRefreshTokenExpiresIn)
			} else {
				bearerToken = &qb.BearerToken{AccessToken: dbCompany.QBBearerToken}
			}
		} else {
			bearerToken, err = qbc.RetrieveBearerToken(req.AuthCode)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not get token", http.StatusInternalServerError)
				return
			}

			err = s.UpsertCompany(req.RealmID, req.AuthCode, bearerToken.AccessToken, bearerToken.ExpiresIn, bearerToken.RefreshToken, bearerToken.XRefreshTokenExpiresIn)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not upsert company", http.StatusInternalServerError)
				return
			}
		}
		qbc.SetClient(*bearerToken)
		userInfo, err := qbc.GetUserInfo()
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not get user info", http.StatusInternalServerError)
			return
		}
		firebaseID, err := s.IsFirebaseUser(req.RealmID)
		if err != nil {
			http.Error(w, "Could not check if user exists", http.StatusInternalServerError)
			return
		}
		if firebaseID == "" {
			// Password is base64 encoded realmID idk man
			encodedRealmID := base64.StdEncoding.EncodeToString([]byte(req.RealmID))
			createdUserResp, err := fbc.SignUp(userInfo.Email, encodedRealmID)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not create user", http.StatusInternalServerError)
				return
			}
			err = s.SetCompanyFirebaseID(req.RealmID, createdUserResp.LocalId)
			if err != nil {
				http.Error(w, "Could not link firebase ID of new user to company", http.StatusInternalServerError)
				return
			}
			firebaseID = createdUserResp.LocalId
		}

		encKey := config.LoadConfigs().JWEKey
		rawKey, err := base64.StdEncoding.DecodeString(encKey)
		if err != nil {
			log.Fatalf("Failed to decode Base64 key: %v", err)
		}
		encrypter, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: rawKey}, nil)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not create encrypter", http.StatusInternalServerError)
			return
		}
		object, err := encrypter.Encrypt([]byte(bearerToken.AccessToken))
		if err != nil {
			http.Error(w, "Could not encrypt token", http.StatusInternalServerError)
			return
		}
		encryptedToken, err := object.CompactSerialize()
		if err != nil {
			http.Error(w, "Could not serialize token", http.StatusInternalServerError)
			return
		}

		customTokenInternal, err := a.CustomTokenWithClaims(r.Context(), firebaseID, map[string]interface{}{
			"qb_company_id":   req.RealmID,
			"qb_bearer_token": encryptedToken,
			"is_franchiser":   true,
			"firebase_id":     firebaseID,
			// For future
			// is_admin
		})
		signInWithCustomTokenResp, err := fbc.SignInWithCustomToken(customTokenInternal)

		// FOR PURELY DEBUGGING PURPOSES
		// decryptedObject, err := jose.ParseEncrypted(encryptedToken)
		// if err != nil {
		// 	log.Fatalf("Failed to parse encrypted message: %v", err)
		// }
		// decrypted, err := decryptedObject.Decrypt(rawKey)
		// if err != nil {
		// 	log.Fatalf("Failed to decrypt message: %v", err)
		// }
		// log.Println(string(decrypted))

		response := response{
			Token:   signInWithCustomTokenResp.IdToken,
			Success: true}
		encode(w, r, 200, response)
	}
}

// // Create a type for all the values I put into the context from the middleware
// // Create a method that retrieves the values from the context, error handles, and returns the value in correct format
// func handlePostFranchise(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
// 	type request struct {
// 		Franchise domain.Franchise `json:"franchise"`
// 	}
// 	type response struct {
// 		id      string `json:"id"`
// 		success bool   `json:"success"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		account_id := r.Context().Value("account_id")
// 		if account_id == nil {
// 			// encode(w, r, 401, response{success: false, id: })
// 			http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 			return
// 		}
// 		accountIDStr, ok := account_id.(string)
// 		if !ok {
// 			http.Error(w, "Invalid account ID", http.StatusBadRequest)
// 			return
// 		}

// 		req, err := decode[request](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}
// 		err = s.CreateFranchise(accountIDStr, req.Franchise)
// 		if err != nil {
// 			// log error
// 			log.Println(err)
// 			http.Error(w, "Could not create franchise", http.StatusInternalServerError)
// 			return
// 		}
// 		// create franchiser user
// 		// user_id, err := s.CreateFranchiseUser()
// 		// write to reponse with response struct
// 		response := response{success: true}
// 		encode(w, r, 200, response)
// 	}
// }

// func handlePostUser(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
// 	// -- 0 for franchisee_non_admin, 1 for franchisee_admin,
// 	// 2 for franchiser_non_admin, 3 for franchiser_admin
// 	type request struct {
// 		Name string `json:"name"`
// 		Role int    `json:"role"`
// 	}
// 	type response struct {
// 		Success bool `json:"success"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		account_id := r.Context().Value("account_id")
// 		if account_id == nil {
// 			// encode(w, r, 401, response{success: false, id: })
// 			http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 			return
// 		}
// 		accountIDStr, ok := account_id.(string)
// 		if !ok {
// 			http.Error(w, "Invalid account ID", http.StatusBadRequest)
// 			return
// 		}
// 		req, err := decode[request](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}
// 		if req.Role == 0 || req.Role == 1 {
// 			// a.EmailSignInLink(r.Context(), req.Name)
// 			// s.CreateFranchiseeUser(req.Name, req.Role)
// 		} else if req.Role == 2 || req.Role == 3 {
// 			franchiseID, err := s.GetFranchiseIDFromAccountId(accountIDStr)
// 			if err != nil {
// 				http.Error(w, "User not designated as franchiser", http.StatusInternalServerError)
// 				return
// 			}
// 			err = s.CreateFranchiseUser(accountIDStr, franchiseID, req.Name)
// 			if err != nil {
// 				http.Error(w, "Could not create user, likely already exists", http.StatusInternalServerError)
// 				return
// 			}
// 		} else {
// 			http.Error(w, "Invalid role", http.StatusBadRequest)
// 			return
// 		}
// 		response := response{Success: true}
// 		encode(w, r, 200, response)
// 	}
// }

// func handleCustomToken(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		account_id := r.Context().Value("account_id")
// 		if account_id == nil {
// 			// encode(w, r, 401, response{success: false, id: })
// 			http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 			return
// 		}
// 		accountIDStr, ok := account_id.(string)
// 		if !ok {
// 			http.Error(w, "Invalid account ID", http.StatusBadRequest)
// 			return
// 		}
// 		// Get User's Role from DB
// 		app_user_id, franchise_id, franchisee_id, role, err := s.GetUserClaims(accountIDStr)
// 		if err != nil {
// 			log.Println(err)
// 			http.Error(w, "Could not get user role", http.StatusInternalServerError)
// 			return
// 		}

// 		token, err := a.CustomTokenWithClaims(r.Context(), accountIDStr, map[string]interface{}{
// 			"app_user_id":   app_user_id,
// 			"franchise_id":  franchise_id,
// 			"franchisee_id": franchisee_id,
// 			"role":          role,
// 		})
// 		if err != nil {
// 			http.Error(w, "Could not create custom token", http.StatusInternalServerError)
// 			return
// 		}
// 		// send custom token back
// 		w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, token)))
// 	}
// }

// func handlePostFranchisee(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
// 	type request struct {
// 		FranchiseeName   string `json:"franchisee_name"`
// 		HeadquartersName string `json:"headquarters_address"`
// 		Email            string `json:"email"`
// 		Phone            string `json:"phone"`
// 	}
// 	type response struct {
// 		Link    string `json:"link"`
// 		Success bool   `json:"success"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		roleIDValue := r.Context().Value("role")
// 		roleIDFloat, ok := roleIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get role ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		role := int(roleIDFloat)

// 		franchiseIDValue := r.Context().Value("franchise_id")
// 		franchiseIDFloat, ok := franchiseIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get franchise ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		franchise_id := int(franchiseIDFloat)

// 		if role != 3 && role != 2 {
// 			http.Error(w, "Requires Franchiser Role", http.StatusUnauthorized)
// 			return
// 		}
// 		req, err := decode[request](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}
// 		// Create Anon User in Firebase and get UID
// 		// Create Franchisee in DB and add UID
// 		// create app_user in DB and link to franchise, franchisee, and firbase uid
// 		u := auth.UserToCreate{}
// 		u.Email(req.Email)
// 		uid, err := a.CreateUser(r.Context(), &u)
// 		if err != nil {
// 			http.Error(w, "Could not create user", http.StatusInternalServerError)
// 			return
// 		}
// 		// TODO: use env variable for URL
// 		emailSetting := auth.ActionCodeSettings{
// 			URL: "https://backend-435201.firebaseapp.com",
// 		}
// 		// a.GetUser(r.Context(), uid.UID)
// 		link, err := a.PasswordResetLinkWithSettings(r.Context(), req.Email, &emailSetting)

// 		// TODO: send email with link.
// 		// Right now admin sdk only let's you create oob link instead of actually sending the email.
// 		// And the shitty thing is that google cloud run doesn't let you send emails
// 		// So unless we figure out another hosting, we'll just have to use external email service for now.

// 		if err != nil {
// 			http.Error(w, "Could not create email link", http.StatusInternalServerError)
// 			return
// 		}

// 		franchisee_id, err := s.CreateFranchisee(franchise_id, req.FranchiseeName, req.HeadquartersName, req.Phone)
// 		if err != nil {
// 			// Delete anon user created if db creation fails
// 			a.DeleteUser(r.Context(), uid.UID)
// 			http.Error(w, "Could not create franchisee", http.StatusInternalServerError)
// 			return
// 		}
// 		err = s.CreateFranchiseeUser(uid.UID, franchise_id, franchisee_id, req.FranchiseeName+" Admin")
// 		if err != nil {
// 			// Delete anon user created if db creation fails
// 			a.DeleteUser(r.Context(), uid.UID)
// 			http.Error(w, "Could not create franchisee", http.StatusInternalServerError)
// 			return
// 		}
// 		response := response{Success: true, Link: link}
// 		encode(w, r, 200, response)
// 	}
// }

// func handlePostProduct(s *storage.SQLStorage) http.HandlerFunc {
// 	// TODO add price santization
// 	type request struct {
// 		ProductName string  `json:"product_name"`
// 		Description string  `json:"description"`
// 		Price       float64 `json:"price"`
// 		// ProductStatus is active by default = 0
// 		// ProductStatus string  `json:"product_status"`
// 	}
// 	type response struct {
// 		ProductID int  `json:"product_id"`
// 		Success   bool `json:"success"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		roleIDValue := r.Context().Value("role")
// 		roleIDFloat, ok := roleIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get role ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		role := int(roleIDFloat)

// 		franchiseIDValue := r.Context().Value("franchise_id")
// 		franchiseIDFloat, ok := franchiseIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get franchise ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		franchise_id := int(franchiseIDFloat)

// 		if role != 3 && role != 2 {
// 			http.Error(w, "Requires Franchiser Role", http.StatusUnauthorized)
// 			return
// 		}
// 		req, err := decode[request](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}

// 		product_id, err := s.CreateProduct(franchise_id, req.ProductName, req.Description, req.Price)
// 		if err != nil {
// 			log.Println(err)
// 			http.Error(w, "Could not create product", http.StatusInternalServerError)
// 			return
// 		}
// 		response := response{Success: true, ProductID: product_id}
// 		encode(w, r, 200, response)
// 	}
// }

// // handleSearchProduct handles the search and listing of products with pagination and sorting.
// func handleSearchProduct(s *storage.SQLStorage) http.HandlerFunc {
// 	allowedSortFields := map[string]bool{
// 		"price":      true,
// 		"created_at": true,
// 		"updated_at": true,
// 	}
// 	allowedSortOrders := map[string]bool{
// 		"asc":  true,
// 		"desc": true,
// 	}
// 	type response struct {
// 		Products      []domain.Product `json:"products"`
// 		NextPageToken string           `json:"next_page_token,omitempty"`
// 	}

// 	return func(w http.ResponseWriter, r *http.Request) {
// 		// TODO right now pagination is based off product_id >,
// 		// So you will have to get to a null page to figure out that you're at the end.
// 		// Is there a better way?
// 		franchiseIdParam := r.Context().Value("franchise_id")
// 		franchiseIdFloat, ok := franchiseIdParam.(float64)
// 		franchiseId := int(franchiseIdFloat)
// 		if !ok {
// 			http.Error(w, "Invalid franchise ID", http.StatusBadRequest)
// 			return
// 		}

// 		var orderBy string
// 		var orderByFields []string
// 		orderByParam := r.URL.Query().Get("order_by")
// 		if orderByParam == "" {
// 			orderBy = "created_at desc"
// 		} else {
// 			orderByFields = strings.Split(orderByParam, ",")
// 			for _, field := range orderByFields {
// 				parts := strings.Split(field, " ")
// 				if len(parts) != 2 {
// 					http.Error(w, "Invalid parameter: order_by", http.StatusBadRequest)
// 					return
// 				}
// 				if !allowedSortFields[parts[0]] || !allowedSortOrders[parts[1]] {
// 					http.Error(w, "Invalid parameter: order_by", http.StatusBadRequest)
// 					return
// 				}
// 			}
// 			orderBy = strings.Join(orderByFields, ", ")
// 		}

// 		query := r.URL.Query().Get("query")

// 		// For now page token doesn't contain continuity information so we just base 64 encode the id
// 		// In the future we can add more informoartion like query and order_by
// 		pageTokenParam := r.URL.Query().Get("page_token")
// 		pageTokenByte, err := base64.StdEncoding.DecodeString(pageTokenParam)
// 		if err != nil {
// 			http.Error(w, "Invalid parameter: page_token", http.StatusBadRequest)
// 			return
// 		}
// 		pageToken := string(pageTokenByte)

// 		// TODO: FIX THIS PAGINATION
// 		// pageToken := string(pageTokenByte)
// 		// pageTokenMap := make(map[string]int)
// 		// err = json.Unmarshal(pageTokenByte, &pageTokenMap)
// 		// if err != nil {
// 		// 	http.Error(w, "Invalid parameter: page_token", http.StatusBadRequest)
// 		// 	return
// 		// }
// 		// Get list of keys of pageTokenMap
// 		// pageTokenKeys := pageTokenMap["keys"]

// 		pageSizeParam := r.URL.Query().Get("page_size")
// 		pageSize, err := strconv.Atoi(pageSizeParam)
// 		if err != nil || pageSize <= 0 {
// 			http.Error(w, "Invalid parameter: page_size", http.StatusBadRequest)
// 			return
// 		}

// 		var products []domain.Product
// 		var nextTokenId string
// 		// log.Println(fmt.Sprintf("%d %s %d %s %s", franchiseId, query, pageSize, pageToken, orderBy))
// 		if query == "" {
// 			// log.Println("List Products")
// 			products, nextTokenId, err = s.ListProducts(franchiseId, pageSize, pageToken, orderBy)
// 			// log.Println(err)
// 		} else {
// 			products, nextTokenId, err = s.SearchProducts(franchiseId, query, pageSize, pageToken, orderBy)
// 			// log.Println(err)
// 		}
// 		// base64 encode the order_by field and nextTokenId into a single string
// 		nextToken := base64.StdEncoding.EncodeToString([]byte(nextTokenId))

// 		if err != nil {
// 			http.Error(w, "Could not get products", http.StatusInternalServerError)
// 			return
// 		}
// 		// Prepare and send the response
// 		resp := response{
// 			Products:      products,
// 			NextPageToken: nextToken,
// 		}
// 		encode(w, r, http.StatusOK, resp)
// 	}
// }

// func handleCreateOrder(s *storage.SQLStorage) http.HandlerFunc {
// 	// Using domain.OrderRequest instead of request struct
// 	// type request struct {
// 	// 	Products []struct {
// 	// 		ProductID int `json:"product_id"`
// 	// 		Quantity  int `json:"quantity"`
// 	// 	} `json:"products"`
// 	// }
// 	type response struct {
// 		OrderID int  `json:"order_id"`
// 		Success bool `json:"success"`
// 	}
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		roleIDValue := r.Context().Value("role")
// 		roleIDFloat, ok := roleIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get role ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		role := int(roleIDFloat)

// 		franchiseIDValue := r.Context().Value("franchise_id")
// 		franchiseIDFloat, ok := franchiseIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get franchise ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		franchise_id := int(franchiseIDFloat)

// 		franchiseeIDValue := r.Context().Value("franchisee_id")
// 		franchiseeIDFloat, ok := franchiseeIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get franchisee ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		franchisee_id := int(franchiseeIDFloat)

// 		appUserIDValue := r.Context().Value("app_user_id")
// 		appUserIDFloat, ok := appUserIDValue.(float64)
// 		if !ok {
// 			http.Error(w, "Cannot get app user ID from JWT", http.StatusBadRequest)
// 			return
// 		}
// 		app_user_id := int(appUserIDFloat)

// 		//  Creator of order should be franchisee
// 		if role == 3 || role == 2 {
// 			http.Error(w, "Requires Franchisee Role", http.StatusUnauthorized)
// 			return
// 		}

// 		order, err := decode[domain.OrderRequest](r)
// 		if err != nil {
// 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 			return
// 		}

// 		var order_id int
// 		order_id, err = s.CreateOrder(app_user_id, franchise_id, franchisee_id, order.Products)
// 		if err != nil {
// 			log.Println(err)
// 			http.Error(w, "Could not create order", http.StatusInternalServerError)
// 			return
// 		}
// 		response := response{Success: true, OrderID: order_id}
// 		encode(w, r, 200, response)
// 	}
// }

// // func createFranchise(s *storage.SQLStorage) http.HandlerFunc {
// // 	type response struct {
// // 		id      string `json:"id"`
// // 		success bool   `json:"success"`
// // 	}
// // 	return func(w http.ResponseWriter, r *http.Request) {
// // 		franchise, err := decode[domain.Franchise](r)
// // 		if err != nil {
// // 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// // 			return
// // 		}
// // 		err := s.createFranchise(franchise)
// // 		if err != nil {
// // 			http.Error(w, "Could not create franchise", http.StatusInternalServerError)
// // 			return
// // 		}
// // 		w.write([]byte(fmt.Sprintf(`{"id": "%s"}`, franchise.Id)))
// // 	}
// // }

// // func createOrder(s *storage.SQLStorage) http.HandlerFunc {
// // 	type request struct {
// // 		Products []string `json:"products"`
// // 	}
// // 	type response struct {
// // 		OrderID string `json:"order_id"`
// // 	}
// // 	return func(w http.ResponseWriter, r *http.Request) {
// // 		var req request
// // 		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// // 			http.Error(w, "Invalid request payload", http.StatusBadRequest)
// // 			return
// // 		}
// // 		// Assuming s.createOrder is a method that creates an order and returns an order ID
// // 		orderID, err := s.CreateOrder(req.Products)
// // 		if err != nil {
// // 			http.Error(w, "Could not create order", http.StatusInternalServerError)
// // 			return
// // 		}
// // 		resp := response{OrderID: orderID}
// // 		w.Header().Set("Content-Type", "application/json")
// // 		json.NewEncoder(w).Encode(resp)
// // 	}
// // }

package net

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/Vertisphere/backend-service/internal/domain"
	"github.com/Vertisphere/backend-service/internal/storage"
	"gopkg.in/square/go-jose.v2"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
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

func ShowClaims() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value("claims").(domain.Claims)
		log.Println(claims)
	}
}

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
			log.Println(dbCompany.QBBearerTokenExpiry)
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
			log.Println(bearerToken)

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
		log.Println(firebaseID)

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
		log.Println(encryptedToken)
		customClaims := domain.Claims{
			QBCompanyID:   req.RealmID,
			QBCustomerID:  "0",
			QBBearerToken: encryptedToken,
			IsFranchiser:  true,
			FirebaseID:    firebaseID,
			// For future
			// IsAdmin
		}
		customTokenInternal, err := a.CustomTokenWithClaims(r.Context(), firebaseID, domain.ClaimsToMap(customClaims))
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not create custom token", http.StatusInternalServerError)
			return
		}
		log.Println(customTokenInternal)
		signInWithCustomTokenResp, err := fbc.SignInWithCustomToken(customTokenInternal)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not sign in with custom token", http.StatusInternalServerError)
			return
		}

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

func ListQBCustomers(qbc *qb.Client) http.HandlerFunc {
	// type request struct {
	// 	OrderBy   string `json:"order_by"`
	// 	PageSize  string `json:"page_size"`
	// 	PageToken string `json:"page_token"`
	// 	Query     string `json:"query"`
	// }
	type response struct {
		Customers []qb.Customer `json:"customers"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get query params
		orderBy := r.URL.Query().Get("order_by")
		pageSize := r.URL.Query().Get("page_size")
		pageToken := r.URL.Query().Get("page_token")
		query := r.URL.Query().Get("query")
		if orderBy == "" {
			orderBy = "DisplayName ASC"
		}
		if pageSize == "" {
			pageSize = "10"
		}
		if pageToken == "" {
			pageToken = "1"
		}
		if query == "" {
			query = ""
		}

		customers, err := qbc.QueryCustomers(claims.QBCompanyID, orderBy, pageSize, pageToken, query)
		if err != nil {
			http.Error(w, "Could not get customers", http.StatusInternalServerError)
			return
		}
		resp := response{Customers: customers}
		encode(w, r, http.StatusOK, resp)
	}
}

func CreateCustomer(fbc *fb.Client, qbc *qb.Client, a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	type request struct {
		QBCustomerID       string `json:"qb_customer_id"`
		CustomerEmail      string `json:"customer_email"`
		SetQBCustomerEmail bool   `json:"set_qb_customer_email"`
	}

	type response struct {
		// This is sent in the email but will put this in for for dev testing
		ResetLink string `json:"reset_link"`
		Success   bool   `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		// TODO add check for existing row in DB
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, req.QBCustomerID)
		if err != nil {
			http.Error(w, "Could not get customer", http.StatusInternalServerError)
			return
		}
		log.Println(customer)
		// Create firebase account for customer
		encodedRealmID := base64.StdEncoding.EncodeToString([]byte(claims.QBCompanyID))
		createdUserResp, err := fbc.SignUp(req.CustomerEmail, encodedRealmID)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not create user", http.StatusInternalServerError)
			return
		}
		log.Println(claims.QBCompanyID)
		err = s.CreateCustomer(claims.QBCompanyID, req.QBCustomerID, createdUserResp.LocalId)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not create customer in DB", http.StatusInternalServerError)
			// TODO: this is kinda faulty
			a.DeleteUser(r.Context(), createdUserResp.LocalId)
			return
		}
		emailSetting := auth.ActionCodeSettings{
			// URL: "https://backend-435201.firebaseapp.com",
			URL: "http://localhost:3000/franchisee/login",
		}
		// a.GetUser(r.Context(), uid.UID)
		link, err := a.PasswordResetLinkWithSettings(r.Context(), req.CustomerEmail, &emailSetting)
		log.Println(link)

		// TODO: clean this shit up
		from := mail.NewEmail("Example User", "sunny@vertisphere.io")
		subject := "Sending with SendGrid is Fun"
		to := mail.NewEmail("Example User", "sunghyoun@icloud.com")
		plainTextContent := "and easy to do anywhere, even with Go"
		htmlContent := link
		message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
		client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
		log.Println(os.Getenv("SENDGRID_API_KEY"))
		resp, err := client.Send(message)
		if err != nil {
			log.Println(err)
			a.DeleteUser(r.Context(), createdUserResp.LocalId)
		}
		log.Println(resp)
		// TODO: IS this 202?
		if resp.StatusCode != http.StatusOK || resp.StatusCode != http.StatusAccepted {
			log.Println("reset email not sent")
			// Actually it was
			// TODO: should we delete this
			// a.DeleteUser(r.Context(), createdUserResp.LocalId)
		}

		// update qb customer to use this email
		if req.SetQBCustomerEmail {
			customerWithEmail := qb.Customer{
				Id:               customer.Id,
				SyncToken:        customer.SyncToken,
				PrimaryEmailAddr: &qb.EmailAddress{Address: req.CustomerEmail},
			}
			_, err = qbc.UpdateCustomer(claims.QBCompanyID, &customerWithEmail)
			if err != nil {
				log.Println(err)
			}
		}

		response := response{Success: true, ResetLink: link}
		encode(w, r, 200, response)
	}
}

func LoginCustomer(fbc *fb.Client, qbc *qb.Client, a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		Token   string `json:"token"`
		Success bool   `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		signInWithPasswordResponse, err := fbc.SignInWithPassword(req.Email, req.Password)
		if err != nil {
			http.Error(w, "Could not sign in", http.StatusInternalServerError)
			return
		}
		log.Println(signInWithPasswordResponse)
		customer, err := s.GetCustomerByFirebaseID(signInWithPasswordResponse.LocalID)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not get customer", http.StatusInternalServerError)
			return
		}
		log.Println(customer)
		dbCompany, err := s.GetCompany(customer.QBCompanyID)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not get company from DB", http.StatusInternalServerError)
			return
		}
		if dbCompany.QBBearerToken == "" {
			http.Error(w, "No cached token", http.StatusBadRequest)
			return
		}
		// Instead of checking for expiry, just get fresh token
		bearerToken, err := qbc.RefreshToken(dbCompany.QBRefreshToken)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not refresh token in DB", http.StatusInternalServerError)
			return
		}
		err = s.UpsertCompany(dbCompany.QBCompanyID, dbCompany.QBAuthCode, bearerToken.AccessToken, bearerToken.ExpiresIn, bearerToken.RefreshToken, bearerToken.XRefreshTokenExpiresIn)

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
		customClaims := domain.Claims{
			QBCompanyID:   dbCompany.QBCompanyID,
			QBCustomerID:  customer.QBCustomerID,
			QBBearerToken: encryptedToken,
			IsFranchiser:  true,
			FirebaseID:    customer.FirebaseID,
			// For future
			// IsAdmin
		}
		customTokenInternal, err := a.CustomTokenWithClaims(r.Context(), customer.FirebaseID, domain.ClaimsToMap(customClaims))
		signInWithCustomTokenResp, err := fbc.SignInWithCustomToken(customTokenInternal)

		response := response{
			Token:   signInWithCustomTokenResp.IdToken,
			Success: true}
		encode(w, r, 200, response)
	}
}
func ListQBItems(qbc *qb.Client) http.HandlerFunc {
	type response struct {
		Items []qb.Item `json:"items"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get query params
		orderBy := r.URL.Query().Get("order_by")
		pageSize := r.URL.Query().Get("page_size")
		pageToken := r.URL.Query().Get("page_token")
		query := r.URL.Query().Get("query")
		if orderBy == "" {
			orderBy = "Name ASC"
		}
		if pageSize == "" {
			pageSize = "10"
		}
		if pageToken == "" {
			pageToken = "1"
		}
		if query == "" {
			query = ""
		}

		items, err := qbc.QueryItems(claims.QBCompanyID, orderBy, pageSize, pageToken, query)
		for _, item := range items {
			// Check if item has field item.SalesTaxRef and then set to 1
			// TODO: set to an actual default defined by franchiser in setting
			// We're just making a guess here and assuming that there is a taxCode 1, but if there isn't, the invoice creation will fail....
			if item.SalesTaxCodeRef.Value == "" {
				item.SalesTaxCodeRef.Value = "1"
			}
		}
		if err != nil {
			http.Error(w, "Could not get items", http.StatusInternalServerError)
			return
		}
		resp := response{Items: items}
		encode(w, r, http.StatusOK, resp)
	}
}
func CreateQBInvoice(qbc *qb.Client) http.HandlerFunc {
	// To be honest the only real values we need from each item is the id, tax code, and price
	// Maybe we should set the tax code to 0 when it's not set and then find the right values for it here?? TODO
	type Line struct {
		Item     qb.Item `json:"item"`
		Quantity float64 `json:"quantity"`
	}
	type request struct {
		Lines []Line `json:"lines"`
	}
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		var lines []qb.Line
		for _, line := range req.Lines {
			unitPrice, err := line.Item.UnitPrice.Float64()
			if err != nil {
				http.Error(w, "Could not convert unit price to float64", http.StatusBadRequest)
				return
			}
			amount := json.Number(strconv.FormatFloat(unitPrice*line.Quantity, 'f', -1, 64))
			lines = append(lines, qb.Line{
				DetailType: "SalesItemLineDetail",
				Amount:     amount,
				SalesItemLineDetail: qb.SalesItemLineDetail{
					ItemRef:    qb.ReferenceType{Value: line.Item.Id},
					TaxCodeRef: qb.ReferenceType{Value: line.Item.SalesTaxCodeRef.Value},
					Qty:        line.Quantity,
				},
			})
		}

		invoice := &qb.Invoice{
			Line:        lines,
			CustomerRef: qb.ReferenceType{Value: claims.QBCustomerID},
			DocNumber:   "P" + time.Now().Format("060102150405") + claims.QBCustomerID,
			// We're going to assume that email is set for customer in qb
			// In fact, I'm going to update the qb customer when we create the franchisee user
			// BillEmail: qb.EmailAddress{Address: },
		}
		qbc.CreateInvoice(claims.QBCompanyID, invoice)
		// add twilio sms messaging
		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
	}
}

func ReviewQBInvoice(qbc *qb.Client) http.HandlerFunc {
	type request struct {
		Approve bool `json:"approve"`
	}
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not get invoice", http.StatusInternalServerError)
			return
		}

		if req.Approve {
			// Calculate the TxnDate and DueDate difference of existingInvoice
			// Then take that difference and add it to the current date
			newDueDate := time.Now().Add(existingInvoice.DueDate.Time.Sub(existingInvoice.TxnDate.Time))
			log.Println(newDueDate)
			// slice old doc number and add R to the front
			newDocNumber := "R" + existingInvoice.DocNumber[1:]
			invoiceToUpdate := struct {
				Id        string `json:"Id"`
				SyncToken string `json:"SyncToken"`
				Sparse    bool   `json:"sparse"`
				DocNumber string `json:"DocNumber"`
				DueDate   string `json:"DueDate"`
			}{
				Id:        invoiceId,
				SyncToken: existingInvoice.SyncToken,
				Sparse:    true,
				DocNumber: newDocNumber,
				DueDate:   newDueDate.Format("2006-01-02"),
			}
			// TODO: Figure out why I'm getting null point on requests using INvoice struct
			// For now just going to initialize my own
			_, err := qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate, existingInvoice.SyncToken)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not update invoice", http.StatusInternalServerError)
				return
			}

			// Send email and notification to customer
			err = qbc.SendInvoice(claims.QBCompanyID, invoiceId, "")
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not send invoice", http.StatusInternalServerError)
				return
			}
		} else {
			newDocNumber := "V" + existingInvoice.DocNumber[1:]
			invoiceToUpdate := qb.Invoice{
				Id:        invoiceId,
				DocNumber: newDocNumber,
			}
			qbc.UpdateInvoice(claims.QBCompanyID, &invoiceToUpdate, existingInvoice.SyncToken)
			// void invoice
			err = qbc.VoidInvoice(claims.QBCompanyID, invoiceId, existingInvoice.SyncToken)
			if err != nil {
				http.Error(w, "Could not void invoice", http.StatusInternalServerError)
				return
			}
		}
	}
}

func ListQBInvoices(qbc *qb.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// Get query params
		orderBy := r.URL.Query().Get("order_by")
		pageSize := r.URL.Query().Get("page_size")
		pageToken := r.URL.Query().Get("page_token")
		// query := r.URL.Query().Get("query")
		if orderBy == "" {
			orderBy = "DocNumber ASC"
		}
		if pageSize == "" {
			pageSize = "10"
		}
		if pageToken == "" {
			pageToken = "1"
		}
		// if query == "" {
		// 	query = ""
		// }

		invoices, err := qbc.QueryInvoices(claims.QBCompanyID, orderBy, pageSize, pageToken)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not get invoices", http.StatusInternalServerError)
			return
		}
		encode(w, r, http.StatusOK, invoices)
	}
}

func ListQBInvoicesCustomer(qbc *qb.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// Get query params
		orderBy := r.URL.Query().Get("order_by")
		pageSize := r.URL.Query().Get("page_size")
		pageToken := r.URL.Query().Get("page_token")
		if orderBy == "" {
			orderBy = "DocNumber ASC"
		}
		if pageSize == "" {
			pageSize = "10"
		}
		if pageToken == "" {
			pageToken = "1"
		}

		invoices, err := qbc.QueryInvoicesCustomer(claims.QBCompanyID, orderBy, pageSize, pageToken, claims.QBCustomerID)
		log.Println(invoices)
		if err != nil {
			http.Error(w, "Could not get invoices", http.StatusInternalServerError)
			return
		}
		encode(w, r, http.StatusOK, invoices)
	}
}

func GetQBCustomer(qbc *qb.Client, s *storage.SQLStorage) http.Handler {
	type response struct {
		Customer qb.Customer `json:"customer"`
		IsLinked bool        `json:"is_linked"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		customerId := r.PathValue("id")
		if customerId == "" {
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, customerId)
		if err != nil {
			http.Error(w, "Could not get customer", http.StatusInternalServerError)
			return
		}
		isLinked, err := s.IsFirebaseUserCustomer(customerId)
		if err != nil {
			http.Error(w, "Could not check if firebase user linked to qb customer", http.StatusInternalServerError)
			return
		}

		resp := response{Customer: *customer, IsLinked: isLinked}
		encode(w, r, http.StatusOK, resp)
	})
}

func GetQBInvoice(qbc *qb.Client) http.Handler {
	type response struct {
		Invoice qb.Invoice `json:"invoice"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}
		invoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not get invoice", http.StatusInternalServerError)
			return
		}
		resp := response{Invoice: *invoice}
		encode(w, r, http.StatusOK, resp)
	})
}

func GetQBInvoicePDF(qbc *qb.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}
		// Get PDF
		pdf, err := qbc.GetInvoicePDF(claims.QBCompanyID, invoiceId)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not get PDF", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=invoice.pdf")
		w.Write(pdf)
	})
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

func decryptJWE(jwe string) (string, error) {
	encKey := config.LoadConfigs().JWEKey
	rawKey, err := base64.StdEncoding.DecodeString(encKey)
	if err != nil {
		return "", err
	}
	decryptedObject, err := jose.ParseEncrypted(jwe)
	if err != nil {
		return "", err
	}
	decrypted, err := decryptedObject.Decrypt(rawKey)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

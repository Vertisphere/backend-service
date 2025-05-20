package net

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/Vertisphere/backend-service/internal/domain"
	"github.com/Vertisphere/backend-service/internal/storage"
	"github.com/twilio/twilio-go"
	twApi "github.com/twilio/twilio-go/rest/api/v2010"
	"gopkg.in/square/go-jose.v2"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/rs/zerolog/log"
)

//	func ShowClaims() http.HandlerFunc {
//		return func(w http.ResponseWriter, r *http.Request) {
//			c := config.LoadConfigs()
//			log.Println("JWEKey:", c.JWEKey)
//			log.Println("PORT:", c.Port)
//			log.Println("DB_HOST:", c.DB.Host)
//			log.Println("DB_USER:", c.DB.User)
//			log.Println("DB_PASS:", c.DB.Password)
//			log.Println("DB_NAME:", c.DB.Name)
//			log.Println("QUICKBOOKS_CLIENT_ID:", c.Quickbooks.ClientID)
//			log.Println("QUICKBOOKS_REDIRECT_URI:", c.Quickbooks.RedirectURI)
//			log.Println("QUICKBOOKS_IS_PRODUCTION:", c.Quickbooks.IsProduction)
//			log.Println("QUICKBOOKS_MINOR_VERSION:", c.Quickbooks.MinorVersion)
//			log.Println("QUICKBOOKS_CLIENT_SECRET:", c.Quickbooks.ClientSecret)
//			log.Println("FIREBASE KEY:", c.Firebase.APIKey)
//			log.Println("JWE_KEY:", c.JWEKey)
//			claims := r.Context().Value("claims").(domain.Claims)
//			log.Println(claims)
//		}
//	}
//
// Get qb token and encrypt it ->
// We get company ID
// Check if company exists in DB
// If exists -> get firebase ID from DB -> Get custom claim Token -> Sign in with custom token -> generate JWT
// If not exists -> create a new firebase anonymous user -> create a new company in DB -> link with firebase user -> generate JWT
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
			logHttpError(err, "Invalid request payload", http.StatusBadRequest, &w)
			return
		}

		// Get QB token
		bearerToken, err := qbc.RetrieveBearerToken(req.AuthCode)
		if err != nil {
			logHttpError(err, "Could not get token", http.StatusInternalServerError, &w)
			return
		}

		// Encrypt token to embed in claims
		encryptedToken, err := encryptToken(bearerToken.AccessToken)
		if err != nil {
			logHttpError(err, "Could not encrypt token", http.StatusInternalServerError, &w)
			return
		}

		// Check if company is in DB
		companyExists, err := s.CompanyExists(req.RealmID)
		if err != nil {
			logHttpError(err, "Could not check if company exists", http.StatusInternalServerError, &w)
			return
		}

		var firebaseID string
		if companyExists {
			// Get company
			company, err := s.GetCompany(req.RealmID)
			if err != nil {
				logHttpError(err, "Could not get company from DB", http.StatusInternalServerError, &w)
				return
			}
			// Get firebase ID (We assume firebase ID is always set)
			firebaseID = company.FirebaseID
		} else {
			// Create a new firebase anonymous user
			userToCreate := auth.UserToCreate{}
			createdUser, err := a.CreateUser(r.Context(), &userToCreate)
			if err != nil {
				logHttpError(err, "Could not create user in firebase", http.StatusInternalServerError, &w)
				return
			}

			log.Debug().Interface("createdUser", createdUser).Msg("Created user in firebase")

			firebaseID = createdUser.UID

			// Create a new company in DB
			err = s.CreateCompany(req.RealmID, req.AuthCode, bearerToken.AccessToken, bearerToken.ExpiresIn, bearerToken.RefreshToken, bearerToken.XRefreshTokenExpiresIn, firebaseID)
			if err != nil {
				// Delete the user we just created
				rollBackErr := a.DeleteUser(r.Context(), firebaseID)
				if rollBackErr != nil {
					log.Error().Err(rollBackErr).Msgf("Could not delete user in firebase after DB createCompany failed for user: %s", firebaseID)
				}
				logHttpError(err, "Could not create company in DB", http.StatusInternalServerError, &w)
				return
			}
		}

		// With firebaseID, create a custom claims
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
			logHttpError(err, "Could not create custom token", http.StatusInternalServerError, &w)
			return
		}
		signInWithCustomTokenResp, err := fbc.SignInWithCustomToken(customTokenInternal)
		if err != nil {
			logHttpError(err, "Could not sign in with custom token", http.StatusInternalServerError, &w)
			return
		}

		response := response{
			Token:   signInWithCustomTokenResp.IdToken,
			Success: true}
		encode(w, r, 200, response)
	}
}

func ListQBCustomers(qbc *qb.Client, s *storage.SQLStorage) http.HandlerFunc {
	type response struct {
		TotalCount int               `json:"total_count"`
		Customers  []domain.Customer `json:"customers"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)

		// If franchisee, then don't allow
		if !claims.IsFranchiser {
			logHttpError(nil, "No Access", http.StatusServiceUnavailable, &w)
			return
		}

		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			logHttpError(err, "Could not decrypt QB token", http.StatusUnauthorized, &w)
			return
		}

		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get query params
		q := r.URL.Query()
		orderBy := getQueryWithDefault(&q, "order_by", "DisplayName ASC")
		pageSize := getQueryWithDefault(&q, "page_size", "10")
		pageToken := getQueryWithDefault(&q, "page_token", "1")
		query := getQueryWithDefault(&q, "query", "DisplayName LIKE '%%'")

		// Get Total Count of query
		totalCount, err := qbc.QueryCustomersCount(claims.QBCompanyID, query)
		if err != nil {
			logHttpError(err, "Could not get customers (total count)", http.StatusInternalServerError, &w)
			return
		}

		// Get Customers from QB
		qbCustomers, err := qbc.QueryCustomers(claims.QBCompanyID, orderBy, pageSize, pageToken, query)
		if err != nil {
			logHttpError(err, "Could not get customers", http.StatusInternalServerError, &w)
			return
		}
		// convert qbCustomers to customer type
		customers := make([]domain.Customer, len(qbCustomers))

		for i, qbCustomer := range qbCustomers {
			customers[i] = domain.Customer{
				Customer: qbCustomer,
			}
		}

		customersWithFirebaseDetails := s.GetCustomersLinkedStatuses(claims.QBCompanyID, &customers)
		// Write customers to response
		resp := response{TotalCount: totalCount, Customers: customersWithFirebaseDetails}
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
		// user should be franchisor
		if !claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}

		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			logHttpError(err, "Could not decrypt QB token", http.StatusUnauthorized, &w)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		req, err := decode[request](r)
		if err != nil {
			logHttpError(err, "Invalid request payload", http.StatusBadRequest, &w)
			return
		}
		// TODO add check for existing row in DB
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, req.QBCustomerID)
		if err != nil {
			logHttpError(err, "Could not get customer", http.StatusInternalServerError, &w)
			return
		}
		// Create firebase account for customer
		encodedRealmID := base64.StdEncoding.EncodeToString([]byte(claims.QBCompanyID))
		phoneNumber := qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)

		createdUserResp, err := fbc.SignUp(req.CustomerEmail, encodedRealmID, phoneNumber)
		if err != nil {
			logHttpError(err, "Could not create user", http.StatusInternalServerError, &w)
			return
		}
		err = s.CreateCustomer(claims.QBCompanyID, req.QBCustomerID, createdUserResp.LocalId)
		if err != nil {
			logHttpError(err, "Could not create customer in DB", http.StatusInternalServerError, &w)
			// TODO: this is kinda faulty
			// TODO long after that todo: what does this even mean
			a.DeleteUser(r.Context(), createdUserResp.LocalId)
			return
		}
		emailSetting := auth.ActionCodeSettings{
			// URL: "https://backend-435201.firebaseapp.com",
			URL: fmt.Sprintf("%s/franchisee/dashboard", os.Getenv("CLIENT_ENDPOINT")),
		}
		// a.GetUser(r.Context(), uid.UID)
		link, err := a.PasswordResetLinkWithSettings(r.Context(), req.CustomerEmail, &emailSetting)
		if err != nil {
			logHttpError(err, "Could not create reset link", http.StatusInternalServerError, &w)
			return
		}

		// TODO: clean this shit up
		from := mail.NewEmail("Ordrport Support", "verification@ordrport.com")
		subject := "Reset your password for Ordrport"
		// TODO: add forgot password flow
		to := mail.NewEmail("Ordrport Franchisee", req.CustomerEmail)
		plainTextContent := "Please reset your password for your Ordrport account"
		htmlContent := link
		message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
		client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
		resp, err := client.Send(message)
		if err != nil {
			log.Error().Err(err)
			a.DeleteUser(r.Context(), createdUserResp.LocalId)
		}
		// TODO: IS this 202?
		if resp.StatusCode != http.StatusAccepted {
			log.Error().Interface("response", resp).Msg("reset email wasn't a 202")
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
				log.Error().Err(err).Msg("Could not update customer email in QB")
			}
		}

		response := response{Success: true, ResetLink: link}
		encode(w, r, 200, response)
	}
}

func DeleteCustomer(a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
	type request struct {
		QBCustomerID string `json:"qb_customer_id"`
	}
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// User should be franchisor
		if !claims.IsFranchiser {
			logHttpError(nil, "No Access", http.StatusServiceUnavailable, &w)
			return
		}
		req, err := decode[request](r)
		if err != nil {
			logHttpError(err, "Invalid request payload", http.StatusBadRequest, &w)
			return
		}

		firebaseID, err := s.DeleteCustomer(claims.QBCompanyID, req.QBCustomerID)
		if err != nil {
			logHttpError(err, "Could not delete customer in db", http.StatusInternalServerError, &w)
			return
		}

		err = a.DeleteUser(r.Context(), firebaseID)
		if err != nil {
			logHttpError(err, "Could not delete user in firebase", http.StatusInternalServerError, &w)
			return
		}

		response := response{Success: true}
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
			logHttpError(err, "Invalid request payload", http.StatusBadRequest, &w)
			return
		}

		signInWithPasswordResponse, err := fbc.SignInWithPassword(req.Email, req.Password)
		if err != nil {
			logHttpError(err, "Could not sign in", http.StatusInternalServerError, &w)
			return
		}
		log.Debug().Interface("signInWithPasswordResponse", signInWithPasswordResponse).Msg("Sign in with password response")
		customer, err := s.GetCustomerByFirebaseID(signInWithPasswordResponse.LocalID)
		if err != nil {
			logHttpError(err, "Could not get customer", http.StatusInternalServerError, &w)
			return
		}
		log.Debug().Interface("customer", customer).Msg("Fetched customer")
		dbCompany, err := s.GetCompany(customer.QBCompanyID)
		if err != nil {
			logHttpError(err, "Could not get company from DB", http.StatusInternalServerError, &w)
			return
		}
		if dbCompany.QBBearerToken == "" {
			logHttpError(nil, "No cached token", http.StatusBadRequest, &w)
			return
		}
		// Instead of checking for expiry, just get fresh token
		bearerToken, err := qbc.RefreshToken(dbCompany.QBRefreshToken)
		if err != nil {
			logHttpError(err, "Could not refresh token in DB", http.StatusInternalServerError, &w)
			return
		}
		err = s.UpsertCompany(dbCompany.QBCompanyID, dbCompany.QBAuthCode, bearerToken.AccessToken, bearerToken.ExpiresIn, bearerToken.RefreshToken, bearerToken.XRefreshTokenExpiresIn)
		if err != nil {
			logHttpError(err, "Could not upsert company", http.StatusInternalServerError, &w)
			return
		}

		encKey := config.LoadConfigs().JWEKey
		rawKey, err := base64.StdEncoding.DecodeString(encKey)
		if err != nil {
			logHttpError(err, "Failed to decode Base64 key", http.StatusInternalServerError, &w)
			return
		}
		encrypter, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: rawKey}, nil)
		if err != nil {
			logHttpError(err, "Could not create encrypter", http.StatusInternalServerError, &w)
			return
		}
		object, err := encrypter.Encrypt([]byte(bearerToken.AccessToken))
		if err != nil {
			logHttpError(err, "Could not encrypt token", http.StatusInternalServerError, &w)
			return
		}
		encryptedToken, err := object.CompactSerialize()
		if err != nil {
			logHttpError(err, "Could not serialize token", http.StatusInternalServerError, &w)
			return
		}
		customClaims := domain.Claims{
			QBCompanyID:   dbCompany.QBCompanyID,
			QBCustomerID:  customer.QBCustomerID,
			QBBearerToken: encryptedToken,
			IsFranchiser:  false,
			FirebaseID:    customer.FirebaseID,
			// For future
			// IsAdmin
		}
		customTokenInternal, err := a.CustomTokenWithClaims(r.Context(), customer.FirebaseID, domain.ClaimsToMap(customClaims))
		if err != nil {
			logHttpError(err, "Could not create custom token", http.StatusInternalServerError, &w)
			return
		}
		signInWithCustomTokenResp, err := fbc.SignInWithCustomToken(customTokenInternal)
		if err != nil {
			logHttpError(err, "Could not sign in with custom token", http.StatusInternalServerError, &w)
			return
		}

		response := response{
			Token:   signInWithCustomTokenResp.IdToken,
			Success: true}
		encode(w, r, 200, response)
	}
}
func ListQBItems(qbc *qb.Client) http.HandlerFunc {
	type response struct {
		TotalCount int       `json:"total_count"`
		Items      []qb.Item `json:"items"`
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
		q := r.URL.Query()
		orderBy := getQueryWithDefault(&q, "order_by", "Name ASC")
		pageSize := getQueryWithDefault(&q, "page_size", "10")
		pageToken := getQueryWithDefault(&q, "page_token", "1")
		query := getQueryWithDefault(&q, "query", "Name LIKE '%%'")

		totalCount, err := qbc.QueryItemsCount(claims.QBCompanyID, query)
		if err != nil {
			logHttpError(err, "Could not get items (total count)", http.StatusInternalServerError, &w)
			return
		}

		items, err := qbc.QueryItems(claims.QBCompanyID, orderBy, pageSize, pageToken, query)
		if err != nil {
			logHttpError(err, "Could not get items", http.StatusInternalServerError, &w)
			return
		}

		resp := response{TotalCount: totalCount, Items: items}
		encode(w, r, http.StatusOK, resp)
	}
}

func CreateQBInvoice(qbc *qb.Client) http.HandlerFunc {
	type response struct {
		Success bool   `json:"success"`
		Id      string `json:"id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to create invoices
		if claims.IsFranchiser {
			logHttpError(nil, "No Access", http.StatusServiceUnavailable, &w)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			logHttpError(err, "Could not decrypt QB token", http.StatusUnauthorized, &w)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Create invoice order details
		var lines []qb.Line
		lines = append(lines, qb.Line{
			DetailType:  "DescriptionOnly",
			Description: "Ordrport Draft: Customer #" + claims.QBCustomerID,
		})

		// Set Docnumber and customer reference for invoice
		invoice := &qb.Invoice{
			Line:        lines,
			CustomerRef: qb.ReferenceType{Value: claims.QBCustomerID},
			DocNumber:   "A1000000-" + time.Now().Format("060102150405"),
		}

		// Get customer details
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, claims.QBCustomerID)
		if err != nil {
			logHttpError(err, "Could not get customer", http.StatusInternalServerError, &w)
		}
		if customer.PrimaryEmailAddr != nil && customer.PrimaryEmailAddr.Address != "" {
			invoice.BillEmail = qb.EmailAddress{Address: customer.PrimaryEmailAddr.Address}
		}

		// Create Invoice and send
		createdInvoice, err := qbc.CreateInvoice(claims.QBCompanyID, invoice)
		if err != nil {
			logHttpError(err, "Could not create invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true, Id: createdInvoice.Id}
		encode(w, r, http.StatusOK, resp)
	}
}

func UpdateQBInvoice(qbc *qb.Client) http.HandlerFunc {
	// To be honest the only realk values we need from each item is the id, tax code, and price
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
		// Franchisers should not be able to create invoices
		if claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in DRAFT
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_DRAFT {
			http.Error(w, "Invoice is not in draft status", http.StatusBadRequest)
			return
		}

		// Decode request
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		// Create invoice order details
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
		// slice old doc number and change status to reviewed
		invoiceToUpdate := struct {
			Id        string    `json:"Id"`
			SyncToken string    `json:"SyncToken"`
			Sparse    bool      `json:"sparse"`
			Line      []qb.Line `json:"Line"`
		}{
			Id:        invoiceId,
			SyncToken: existingInvoice.SyncToken,
			Sparse:    true,
			Line:      lines,
		}
		_, err = qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate)
		if err != nil {
			logHttpError(err, "Could not update invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
	}
}

func PublishQBInvoice(qbc *qb.Client, auth *auth.Client, twc *twilio.RestClient) http.HandlerFunc {
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to create invoices
		if claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in DRAFT
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_DRAFT {
			http.Error(w, "Invoice is not in draft status", http.StatusBadRequest)
			return
		}
		// slice old doc number and change status to reviewed
		invoiceToUpdate := struct {
			Id        string `json:"Id"`
			SyncToken string `json:"SyncToken"`
			Sparse    bool   `json:"sparse"`
			DocNumber string `json:"DocNumber"`
		}{
			Id:        invoiceId,
			SyncToken: existingInvoice.SyncToken,
			Sparse:    true,
			DocNumber: qb.ChangeInvoiceStatus(existingInvoice.DocNumber, qb.INVOICE_PENDING),
		}
		_, err = qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate)
		if err != nil {
			logHttpError(err, "Could not update invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)

		// Messaging isn't as important so we send message after we send response
		// TODO: add twilio sms messaging
		// add twilio message
		// Do we get the phone number from the customer or the user??? Going to be using firebase phone number for now
		// Okay here's how we're setting priority:
		// 1. Check MfaEnrollment PhoneNumber (aka the phone number set for MFA)
		// if none exist then: 2. Check firebase.UserInfo.PhoneNumber (What got set when firebase account was created)
		// if none exist then: 3. Check qb customer phone
		// Get firebase user phone number
		// user, err := a.GetUser(r.Context(), claims.FirebaseID)
		// if err != nil {
		// 	log.Println(err)
		// }
		// // TODO: implement get mfa enrollment in fbc
		// // 1. get mfaInfo
		// // phoneNumber := fbc.GetUserInfo.MfaEnrollment.PhoneNumber
		// var phoneNumber string
		// // 2.
		// if phoneNumber == "" {
		// 	phoneNumber = user.PhoneNumber
		// }
		// 3.

		// get phone number for franchisor to send notification to
		var phoneNumber string
		franchisor, err := qbc.FindCompanyInfo(claims.QBCompanyID)
		// get customer name
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		if err != nil {
			log.Error().Err(err).Msg("Could not get company or customer to send sms message for publish")
		} else {
			if phoneNumber == "" {
				phoneNumber = qbToE164Phone(franchisor.PrimaryPhone.FreeFormNumber)
			}
		}

		// TODO FOR Demo purpose
		// if phoneNumber == "" {
		// 	log.Println("No Phone number to send notification to. None sent.")
		// 	return
		// }
		// TODO are all these phone number formats the same?
		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("An order with ID number %s has been published by franchisee: %s. For more details please visit: https://ordrport.com/franchisor/orders/pending-review", existingInvoice.Id, customer.DisplayName))
		params.SetFrom("+16478009984")
		params.SetTo(phoneNumber)

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Error().Interface("twResp", *twResp.Body).Msg("Could not send SMS message")
			} else {
				log.Print(twResp)
			}
		}
	}
}

func UnpublishQBInvoice(qbc *qb.Client, auth *auth.Client, twc *twilio.RestClient) http.HandlerFunc {
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in PENDING
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_PENDING {
			http.Error(w, "Invoice is not in pending status", http.StatusBadRequest)
			return
		}
		// slice old doc number and change status to reviewed
		invoiceToUpdate := struct {
			Id        string `json:"Id"`
			SyncToken string `json:"SyncToken"`
			Sparse    bool   `json:"sparse"`
			DocNumber string `json:"DocNumber"`
		}{
			Id:        invoiceId,
			SyncToken: existingInvoice.SyncToken,
			Sparse:    true,
			DocNumber: qb.ChangeInvoiceStatus(existingInvoice.DocNumber, qb.INVOICE_DRAFT),
		}
		_, err = qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate)
		if err != nil {
			logHttpError(err, "Could not update invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
		// Messaging isn't as important so we send message after we send response
		// TODO: add twilio sms messaging
		// add twilio message
		// Do we get the phone number from the customer or the user??? Going to be using firebase phone number for now
		// Okay here's how we're setting priority:
		// 1. Check MfaEnrollment PhoneNumber (aka the phone number set for MFA)
		// if none exist then: 2. Check firebase.UserInfo.PhoneNumber (What got set when firebase account was created)
		// if none exist then: 3. Check qb customer phone
		// Get firebase user phone number
		// user, err := a.GetUser(r.Context(), claims.FirebaseID)
		// if err != nil {
		// 	log.Println(err)
		// }
		// // TODO: implement get mfa enrollment in fbc
		// // 1. get mfaInfo
		// // phoneNumber := fbc.GetUserInfo.MfaEnrollment.PhoneNumber
		// var phoneNumber string
		// // 2.
		// if phoneNumber == "" {
		// 	phoneNumber = user.PhoneNumber
		// }
		// // 3.

		var phoneNumber string
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		if err != nil {
			log.Error().Err(err).Msg("Could not get customer to send sms message for unpublish")
		} else {
			if phoneNumber == "" {
				phoneNumber = qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
				if customer.PrimaryPhone.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.Mobile.FreeFormNumber)
				}
				if customer.Mobile.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.AlternatePhone.FreeFormNumber)
				}
				if err != nil {
					log.Printf("%s", err)
				}
			}
		}

		// TODO FOR Demo purpose
		if phoneNumber == "" {
			log.Error().Msg("No phone number to send notification to. None sent.")
			return
		}
		// TODO are all these phone number formats the same?
		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("An order with ID number %s has been rejected. For more details please visit: https://ordrport.com/franchisor/orders/pending-review", existingInvoice.Id))
		params.SetFrom("+16478009984")
		params.SetTo(phoneNumber)

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Error().Interface("twResp", twResp).Msg("twRespbody was nil")
			} else {
				log.Print(twResp)
			}
		}
	}
}

func ApproveQBInvoice(qbc *qb.Client, a *auth.Client, twc *twilio.RestClient) http.HandlerFunc {
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to create invoices
		if !claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in PENDING
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_PENDING {
			http.Error(w, "Invoice is not in pending status", http.StatusBadRequest)
			return
		}
		// slice old doc number and change status to reviewed
		invoiceToUpdate := struct {
			Id        string `json:"Id"`
			SyncToken string `json:"SyncToken"`
			Sparse    bool   `json:"sparse"`
			DocNumber string `json:"DocNumber"`
		}{
			Id:        invoiceId,
			SyncToken: existingInvoice.SyncToken,
			Sparse:    true,
			DocNumber: qb.ChangeInvoiceStatus(existingInvoice.DocNumber, qb.INVOICE_APPROVED),
		}
		_, err = qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate)
		if err != nil {
			logHttpError(err, "Could not update invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
		// Messaging isn't as important so we send message after we send response
		// TODO: add twilio sms messaging
		// add twilio message
		// Do we get the phone number from the customer or the user??? Going to be using firebase phone number for now
		// Okay here's how we're setting priority:
		// 1. Check MfaEnrollment PhoneNumber (aka the phone number set for MFA)
		// if none exist then: 2. Check firebase.UserInfo.PhoneNumber (What got set when firebase account was created)
		// if none exist then: 3. Check qb customer phone
		// Get firebase user phone number
		// user, err := a.GetUser(r.Context(), claims.FirebaseID)
		// if err != nil {
		// 	log.Println(err)
		// }
		// TODO: implement get mfa enrollment in fbc
		// 1. get mfaInfo
		// phoneNumber := fbc.GetUserInfo.MfaEnrollment.PhoneNumber
		var phoneNumber string
		// 2.
		// if phoneNumber == "" {
		// 	phoneNumber = user.PhoneNumber
		// }
		// // 3.

		customer, err := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		if err != nil {
			log.Printf("%s", err)
		} else {
			if phoneNumber == "" {
				phoneNumber = qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
				if customer.PrimaryPhone.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.Mobile.FreeFormNumber)
				}
				if customer.Mobile.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.AlternatePhone.FreeFormNumber)
				}
				if err != nil {
					log.Printf("%s", err)
				}
			}
		}

		// TODO FOR Demo purpose
		// if phoneNumber == "" {
		// 	log.Println("No Phone number to send notification to. None sent.")
		// 	return
		// }
		// TODO are all these phone number formats the same?
		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("Order #%s has been approved and is being prepared. For more details please visit: https://ordrport.com/franchisee/orders", invoiceId))
		// Fuck it we hard code the phone number (for now)
		params.SetFrom("+16478009984")
		params.SetTo(phoneNumber)

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Error().Interface("twResp", twResp).Msg("twRespbody was nil")
			} else {
				log.Print(twResp)
			}
		}
	}
}

func VoidQBInvoice(qbc *qb.Client, a *auth.Client, twc *twilio.RestClient) http.HandlerFunc {
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to void invoices
		if claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in PENDING
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_PENDING && qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_DRAFT {
			http.Error(w, "Invoice is not in pending or draft status", http.StatusBadRequest)
			return
		}
		// slice old doc number and change status to voided
		invoiceToUpdate := struct {
			Id        string `json:"Id"`
			SyncToken string `json:"SyncToken"`
			Sparse    bool   `json:"sparse"`
			DocNumber string `json:"DocNumber"`
		}{
			Id:        invoiceId,
			SyncToken: existingInvoice.SyncToken,
			Sparse:    true,
			DocNumber: qb.ChangeInvoiceStatus(existingInvoice.DocNumber, qb.INVOICE_VOID),
		}
		updatedInvoice, err := qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate)
		err = qbc.VoidInvoice(claims.QBCompanyID, updatedInvoice.Id, updatedInvoice.SyncToken)
		if err != nil {
			logHttpError(err, "Could not void invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
		// send twilio message that invoice has been voided
		user, err := a.GetUser(r.Context(), claims.FirebaseID)
		if err != nil {
			log.Error().Err(err).Msg("Could not get user to send sms message for void")
		}
		var phoneNumber string
		if phoneNumber == "" {
			phoneNumber = user.PhoneNumber
		}
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		if err != nil {
			log.Printf("%s", err)
		} else {
			if phoneNumber == "" {
				phoneNumber = qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
				if customer.PrimaryPhone.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.Mobile.FreeFormNumber)
				}
				if customer.Mobile.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.AlternatePhone.FreeFormNumber)
				}

				if err != nil {
					log.Printf("%s", err)
				}
			}
		}

		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("Order %s has been voided by: %s. For more details please visit: https://ordrport.com/franchisee/invoices", customer.DisplayName, invoiceId))
		params.SetFrom("+16478009984")
		params.SetTo("+15062329415")

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Error().Interface("twResp", twResp).Msg("twRespbody was nil")
			} else {
				log.Print(twResp)
			}
		}
	}
}

func DeleteQBInvoice(qbc *qb.Client, a *auth.Client, twc *twilio.RestClient) http.HandlerFunc {
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to delete invoices
		if claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in DRAFT
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_DRAFT {
			http.Error(w, "Invoice is not in pending or draft status", http.StatusBadRequest)
			return
		}
		err = qbc.VoidInvoice(claims.QBCompanyID, existingInvoice.Id, existingInvoice.SyncToken)
		if err != nil {
			logHttpError(err, "Could not void invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
		// send twilio message that invoice has been voided
		user, err := a.GetUser(r.Context(), claims.FirebaseID)
		if err != nil {
			log.Error().Err(err).Msg("Could not get user to send sms message for void")
		}
		var phoneNumber string
		if phoneNumber == "" {
			phoneNumber = user.PhoneNumber
		}
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		if err != nil {
			log.Printf("%s", err)
		} else {
			if phoneNumber == "" {
				phoneNumber = qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
				if customer.PrimaryPhone.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.Mobile.FreeFormNumber)
				}
				if customer.Mobile.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.AlternatePhone.FreeFormNumber)
				}

				if err != nil {
					log.Printf("%s", err)
				}
			}
		}

		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("Order %s has been voided by: %s. For more details please visit: https://ordrport.com/franchisee/invoices", customer.DisplayName, invoiceId))
		params.SetFrom("+16478009984")
		params.SetTo("+15062329415")

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Error().Interface("twResp", twResp).Msg("twRespbody was nil")
			} else {
				log.Print(twResp)
			}
		}
	}
}

func CompleteQBInvoice(qbc *qb.Client, a *auth.Client, twc *twilio.RestClient, s *storage.SQLStorage) http.HandlerFunc {
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to complete invoices
		if !claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})

		// get id from url
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No ID in URL", http.StatusBadRequest)
			return
		}
		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}
		// Verify that the invoice status is currently in APPROVED
		if qb.CheckInvoiceStatus(existingInvoice) != qb.INVOICE_APPROVED {
			http.Error(w, "Invoice is not in approved status", http.StatusBadRequest)
			return
		}
		// slice old doc number and change status to completed
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
			DocNumber: qb.ChangeInvoiceStatus(existingInvoice.DocNumber, qb.INVOICE_COMPLETE),
			// 2 weeks from now by default
			DueDate: time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
		}
		_, err = qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate)
		if err != nil {
			logHttpError(err, "Could not update invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)

		// Basic debug observability
		log.Debug().Interface("invoice", existingInvoice).Msg("Invoice completed")
		log.Debug().Msg("Customer Name: " + existingInvoice.CustomerRef.Name)

		// Send email with invoice
		// companyName := "OrdrPort Franchisor #" + claims.QBCompanyID
		// customerName := "Customer " + existingInvoice.CustomerRef.Name
		companyName := existingInvoice.BillAddr.Line2
		customerName := existingInvoice.BillAddr.Line1
		customerEmail := existingInvoice.BillEmail.Address

		// Get Customer information from QB
		// qbCustomer, qbErr := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		// if qbErr != nil {
		// 	log.Error().Err(qbErr).Msg("Could not get customer information from QB to send email")
		// } else {
		// companyName = existingInvoice.BillAddr.Line2
		// customerName = qbCustomer.DisplayName
		// }

		// Get customer information from DB
		// dbCustomer, dbErr := s.GetCustomerByQBID(existingInvoice.CustomerRef.Value, claims.QBCompanyID)
		// if dbErr != nil {
		// 	log.Error().Err(dbErr).Msg("Could not get customer information from DB to send email")
		// } else {
		// 	fbCustomer, fbErr := a.GetUser(r.Context(), dbCustomer.FirebaseID)
		// 	if fbErr != nil {
		// 		log.Error().Err(fbErr).Msg("Could not get customer information from Firebase to send email")
		// 	} else {
		// 		customerEmail = fbCustomer.Email
		// 		// If we can't get the customer from QB, we use the customer from DB
		// 		if qbErr != nil {
		// 			customerName = fbCustomer.DisplayName
		// 		}
		// 	}
		// }

		// Subject and content
		subject := fmt.Sprintf("Your Order %s is Ready for Pickup!", invoiceId)
		htmlContent := fmt.Sprintf("<html><body><h1>Hello %s,</h1><p>Your order %s is ready for pickup!</p><p>To see the invoice, please visit: <a href=\"https://ordrport.com/franchisee/invoices/%s\">Invoice</a></p><p>Thank you for using OrdrPort!</p><p>Best regards,<br>%s</p></body></html>", customerName, invoiceId, invoiceId, companyName)
		content := mail.NewContent("text/html", htmlContent)

		// Get Invoice PDF
		a_pdf := mail.NewAttachment()
		pdf, err := qbc.GetInvoicePDF(claims.QBCompanyID, invoiceId)
		if err != nil {
			logHttpError(err, "Could not get PDF", http.StatusInternalServerError, &w)
			return
		}
		encoded := base64.StdEncoding.EncodeToString(pdf)
		a_pdf.SetContent(encoded)
		a_pdf.SetType("application/pdf")
		a_pdf.SetFilename(fmt.Sprintf("Invoice_%s.pdf", invoiceId))
		a_pdf.SetDisposition("attachment")

		// Create attachments
		attachments := []*mail.Attachment{a_pdf}

		sendEmail(companyName, customerName, customerEmail, subject, content, attachments)

		// Messaging isn't as important so we send message after we send response
		// TODO: add twilio sms messaging
		// add twilio message
		// Do we get the phone number from the customer or the user??? Going to be using firebase phone number for now
		// Okay here's how we're setting priority:
		// 1. Check MfaEnrollment PhoneNumber (aka the phone number set for MFA)
		// if none exist then: 2. Check firebase.UserInfo.PhoneNumber (What got set when firebase account was created)
		// if none exist then: 3. Check qb customer phone
		// Get firebase user phone number
		user, err := a.GetUser(r.Context(), claims.FirebaseID)
		if err != nil {
			log.Error().Err(err).Msg("Could not get user")
		}
		// TODO: implement get mfa enrollment in fbc
		// 1. get mfaInfo
		// phoneNumber := fbc.GetUserInfo.MfaEnrollment.PhoneNumber
		var phoneNumber string
		// 2.
		if phoneNumber == "" {
			phoneNumber = user.PhoneNumber
		}
		// 3.
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, existingInvoice.CustomerRef.Value)
		if err != nil {
			log.Printf("%s", err)
		} else {
			if phoneNumber == "" {
				phoneNumber = qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
				if customer.PrimaryPhone.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.Mobile.FreeFormNumber)
				}
				if customer.Mobile.FreeFormNumber == "" {
					phoneNumber = qbToE164Phone(customer.AlternatePhone.FreeFormNumber)
				}

				if err != nil {
					log.Printf("%s", err)
				}
			}
		}

		// TODO FOR Demo purpose
		// if phoneNumber == "" {
		// 	log.Println("No Phone number to send notification to. None sent.")
		// 	return
		// }
		// TODO are all these phone number formats the same?
		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("Order %s has been completed and is ready for pickup! To see the invoice, please visit: https://ordrport.com/franchisee/invoices/%s", invoiceId, invoiceId))
		// Fuck it we hard code the phone number (for now)
		params.SetFrom("+16478009984")
		params.SetTo(phoneNumber)

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Error().Interface("twResp", twResp).Msg("twRespbody was nil")
			} else {
				log.Print(twResp)
			}
		}

		// fuinally send email as well
		// if customer.PrimaryEmailAddr != nil && customer.PrimaryEmailAddr.Address != "" {
		// 	err = qbc.SendInvoice(claims.QBCompanyID, invoiceId, customer.PrimaryEmailAddr.Address)
		// 	if err != nil {
		// 		log.Error().Err(err).Msg("Email couldn't be sent")
		// 	}
		// }
	}
}

func DuplicateQBInvoice(qbc *qb.Client) http.HandlerFunc {
	type response struct {
		Success bool   `json:"success"`
		Id      string `json:"id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisers should not be able to create invoices
		if claims.IsFranchiser {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}

		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}

		// Get invoice by ID
		existingInvoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not verify that invoice exists or is in draft status", http.StatusInternalServerError)
			return
		}

		// Get existing invoice lines

		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Create invoice order details

		// Set Docnumber and customer reference for invoice
		invoice := &qb.Invoice{
			Line:        existingInvoice.Line,
			CustomerRef: qb.ReferenceType{Value: claims.QBCustomerID},
			DocNumber:   "A1000000-" + time.Now().Format("060102150405"),
		}

		// Get customer details
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, claims.QBCustomerID)
		if err != nil {
			log.Printf("%s", err)
		}
		if customer.PrimaryEmailAddr != nil && customer.PrimaryEmailAddr.Address != "" {
			invoice.BillEmail = qb.EmailAddress{Address: customer.PrimaryEmailAddr.Address}
		}

		// Create Invoice and send
		createdInvoice, err := qbc.CreateInvoice(claims.QBCompanyID, invoice)
		if err != nil {
			logHttpError(err, "Could not create invoice", http.StatusInternalServerError, &w)
			return
		}

		resp := response{Success: true, Id: createdInvoice.Id}
		encode(w, r, http.StatusOK, resp)
	}
}

func ListQBInvoices(qbc *qb.Client) http.HandlerFunc {
	type response struct {
		TotalCount int                   `json:"total_count"`
		Invoices   []qb.InvoiceTruncated `json:"invoices"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get query params
		q := r.URL.Query()
		orderBy := getQueryWithDefault(&q, "order_by", "DocNumber ASC")
		pageSize := getQueryWithDefault(&q, "page_size", "10")
		pageToken := getQueryWithDefault(&q, "page_token", "1")
		statuses := getQueryWithDefault(&q, "statuses", "DPARVC")
		customerRef := r.URL.Query().Get("customer_ref")
		query := getQueryWithDefault(&q, "query", "")
		// Because quickbooks doesn't allow to query invoices by name directly, first get ids of customers based on query, and then get invoices for those customers
		if !claims.IsFranchiser {
			customerRef = claims.QBCustomerID
		}

		totalCount, err := qbc.QueryInvoicesCount(claims.QBCompanyID, statuses, customerRef, query)
		if err != nil {
			http.Error(w, "Could not get customers (total count)", http.StatusInternalServerError)
			return
		}

		invoices, err := qbc.QueryInvoices(claims.QBCompanyID, orderBy, pageSize, pageToken, statuses, customerRef, query)
		if err != nil {
			logHttpError(err, "Could not get invoices", http.StatusInternalServerError, &w)
			return
		}

		resp := response{TotalCount: totalCount, Invoices: invoices}
		encode(w, r, http.StatusOK, resp)
	}
}

func GetQBCustomer(qbc *qb.Client, s *storage.SQLStorage) http.Handler {
	type response struct {
		Customer qb.Customer `json:"customer"`
		IsLinked bool        `json:"is_linked"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get claims from jwt
		claims := r.Context().Value("claims").(domain.Claims)
		// Decrypt JWE for QB from JWT
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get Customer ID from URL
		customerId := r.PathValue("id")
		if customerId == "" {
			logHttpError(err, "No ID in URL", http.StatusBadRequest, &w)
			return
		}
		// If franchisee is calling, they can get only get information about themselves
		if !claims.IsFranchiser && customerId != claims.QBCustomerID {
			logHttpError(err, "No Access", http.StatusServiceUnavailable, &w)
			return
		}
		// Get customer from QB
		customer, err := qbc.GetCustomerById(claims.QBCompanyID, customerId)
		if err != nil {
			http.Error(w, "Could not get customer", http.StatusInternalServerError)
			return
		}
		// Check if firebase account exists for QB user
		isLinked, err := s.IsFirebaseUserCustomer(customerId)
		if err != nil {
			logHttpError(err, "Could not check if firebase user linked to qb customer", http.StatusInternalServerError, &w)
			return
		}
		// Return customer and if linked
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
		// Get QB token and set jwt for QB Client
		token, err := decryptJWE(claims.QBBearerToken)
		if err != nil {
			logHttpError(err, "Could not decrypt QB token", http.StatusUnauthorized, &w)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get query params
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			logHttpError(err, "No ID in URL", http.StatusBadRequest, &w)
			return
		}
		// Get Invoice from QB
		invoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			logHttpError(err, "Could not get invoice", http.StatusInternalServerError, &w)
			return
		}
		// If franchisee is calling, they can get only get information about themselves
		if !claims.IsFranchiser && invoice.CustomerRef.Value != claims.QBCustomerID {
			logHttpError(err, "No Access", http.StatusServiceUnavailable, &w)
			return
		}
		// Write invoice to response
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
			logHttpError(err, "Could not get PDF", http.StatusInternalServerError, &w)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=invoice.pdf")
		w.Write(pdf)
	})
}

func getQueryWithDefault(q *url.Values, field string, fallback string) string {
	val := q.Get(field)
	if val == "" {
		return fallback
	} else {
		return val
	}
}

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

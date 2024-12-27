package net

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
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
)

func ShowClaims() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := config.LoadConfigs()
		log.Println("JWEKey:", c.JWEKey)
		log.Println("PORT:", c.Port)
		log.Println("DB_HOST:", c.DB.Host)
		log.Println("DB_USER:", c.DB.User)
		log.Println("DB_PASS:", c.DB.Password)
		log.Println("DB_NAME:", c.DB.Name)
		log.Println("QUICKBOOKS_CLIENT_ID:", c.Quickbooks.ClientID)
		log.Println("QUICKBOOKS_REDIRECT_URI:", c.Quickbooks.RedirectURI)
		log.Println("QUICKBOOKS_IS_PRODUCTION:", c.Quickbooks.IsProduction)
		log.Println("QUICKBOOKS_MINOR_VERSION:", c.Quickbooks.MinorVersion)
		log.Println("QUICKBOOKS_CLIENT_SECRET:", c.Quickbooks.ClientSecret)
		log.Println("FIREBASE KEY:", c.Firebase.APIKey)
		log.Println("JWE_KEY:", c.JWEKey)
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

			// TODO: return a transaction here that we can rollback if something goes wrong
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
			// TODO rollback
			log.Println(err)
			http.Error(w, "Could not get user info", http.StatusInternalServerError)
			return
		}
		firebaseID, err := s.IsFirebaseUser(req.RealmID)
		if err != nil {
			// TODO rollback
			http.Error(w, "Could not check if user exists", http.StatusInternalServerError)
			return
		}
		if firebaseID == "" {
			// Password is base64 encoded realmID idk man
			encodedRealmID := base64.StdEncoding.EncodeToString([]byte(req.RealmID))
			// TODO: if userInfo.PhoneNumber is not defined is it ""?
			// Correct format for Firebase Auth
			// phoneNumber := "+19099090900"  // E.164 format)
			phoneNumber, err := qbToE164Phone(userInfo.PhoneNumber)
			if err != nil {
				phoneNumber = ""
			}
			createdUserResp, err := fbc.SignUp(userInfo.Email, encodedRealmID, phoneNumber)
			if err != nil {
				// TODO rollback transaction and delete firebase uesr
				log.Println(err)
				http.Error(w, "Could not create user", http.StatusInternalServerError)
				return
			}
			err = s.SetCompanyFirebaseID(req.RealmID, createdUserResp.LocalId)
			if err != nil {
				// TODO rollback transaction and delete firebase user
				http.Error(w, "Could not link firebase ID of new user to company", http.StatusInternalServerError)
				return
			}
			firebaseID = createdUserResp.LocalId
		}
		log.Println(firebaseID)

		encKey := config.LoadConfigs().JWEKey
		rawKey, err := base64.StdEncoding.DecodeString(encKey)
		if err != nil {
			// rollback
			log.Fatalf("Failed to decode Base64 key: %v", err)
		}
		encrypter, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: rawKey}, nil)
		if err != nil {
			// rollback
			log.Println(err)
			http.Error(w, "Could not create encrypter", http.StatusInternalServerError)
			return
		}
		object, err := encrypter.Encrypt([]byte(bearerToken.AccessToken))
		if err != nil {
			// rollback
			http.Error(w, "Could not encrypt token", http.StatusInternalServerError)
			return
		}
		encryptedToken, err := object.CompactSerialize()
		if err != nil {
			// rollback
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

		// TODO return name for intro
		response := response{
			Token:   signInWithCustomTokenResp.IdToken,
			Success: true}
		encode(w, r, 200, response)
	}
}

func ListQBCustomers(qbc *qb.Client, s *storage.SQLStorage) http.HandlerFunc {
	type response struct {
		Customers []domain.Customer `json:"customers"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// If franchisee, then don't allow
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
		// Get query params
		q := r.URL.Query()
		orderBy := getQueryWithDefault(&q, "order_by", "DisplayName ASC")
		// pageSize := getQueryWithDefault(&q, "order_by", "")
		// pageToken :=
		// orderBy := r.URL.Query().Get("order_by")
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
			query = "DisplayName LIKE '%%'"
		}
		// Get Customers from QB
		qbCustomers, err := qbc.QueryCustomers(claims.QBCompanyID, orderBy, pageSize, pageToken, query)
		if err != nil {
			http.Error(w, "Could not get customers", http.StatusInternalServerError)
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
		resp := response{Customers: customersWithFirebaseDetails}
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
		phoneNumber, err := qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
		if err != nil {
			phoneNumber = ""
		}
		log.Println(phoneNumber)
		createdUserResp, err := fbc.SignUp(req.CustomerEmail, encodedRealmID, phoneNumber)
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
		from := mail.NewEmail("Ordrport Support", "sunny@vertisphere.io")
		subject := "Password Reset for Ordrport"
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
			log.Println(err)
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
			IsFranchiser:  false,
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
func CreateQBInvoice(qbc *qb.Client, twc *twilio.RestClient, a *auth.Client) http.HandlerFunc {
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
		// Set Docnumber and customer reference for invoice
		invoice := &qb.Invoice{
			Line:        lines,
			CustomerRef: qb.ReferenceType{Value: claims.QBCustomerID},
			DocNumber:   "A1000000-" + time.Now().Format("060102150405"),
			// We're going to assume that email is set for customer in qb
			// In fact, I'm going to update the qb customer when we create the franchisee user
			// BillEmail: qb.EmailAddress{Address: },
		}
		// Create Invoice and send
		invoiceResp, err := qbc.CreateInvoice(claims.QBCompanyID, invoice)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not create invoice", http.StatusInternalServerError)
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
		user, err := a.GetUser(r.Context(), claims.FirebaseID)
		if err != nil {
			log.Println(err)
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

		customer, err := qbc.GetCustomerById(claims.QBCompanyID, claims.QBCustomerID)
		if err != nil {
			log.Printf("%s", err)
		}
		if phoneNumber == "" {
			log.Println(customer.PrimaryPhone.FreeFormNumber)
			phoneNumber, err = qbToE164Phone(customer.PrimaryPhone.FreeFormNumber)
			if err != nil {
				log.Printf("%s", err)
			}
		}
		// TODO FOR Demo purpose
		// if phoneNumber == "" {
		// 	log.Println("No Phone number to send notification to. None sent.")
		// 	return
		// }
		// TODO are all these phone number formats the same?
		params := &twApi.CreateMessageParams{}
		params.SetBody(fmt.Sprintf("A new order %s has been created by: %s. For more details please visit: https://ordrport.com/franchiser/invoices", customer.DisplayName, invoiceResp.Id))
		// Fuck it we hard code the phone number (for now)
		params.SetFrom("+16478009984")
		params.SetTo("+15062329415")

		twResp, err := twc.Api.CreateMessage(params)
		if err != nil {
			log.Printf("Could not send SMS message %s", err)
		} else {
			if twResp.Body != nil {
				log.Println(*twResp.Body)
			} else {
				log.Println(twResp.Body)
			}
		}

	}
}

func UpdateQBInvoice(qbc *qb.Client, twc *twilio.RestClient) http.HandlerFunc {
	type request struct {
		Status string `json:"status"`
	}
	type response struct {
		Success bool `json:"success"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// get claims from context
		claims := r.Context().Value("claims").(domain.Claims)
		// Franchisee cannot review invoice
		if !claims.IsFranchiser {
			http.Error(w, "No access", http.StatusServiceUnavailable)
			return
		}
		// Get QB token and set jwt for QB Client
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

		if req.Status == "R" {
			if len(existingInvoice.DocNumber) < 8 || existingInvoice.DocNumber[0:8] != "A1000000" {
				http.Error(w, "Invoice is not in pending state. Verify that invoice has not been already been reviewed and was created from ordrport.", http.StatusBadRequest)
				return
			}
			// Calculate the TxnDate and DueDate difference of existingInvoice
			// Then take that difference and add it to the current date
			newDueDate := time.Now().Add(existingInvoice.DueDate.Time.Sub(existingInvoice.TxnDate.Time))
			log.Println(newDueDate)
			// slice old doc number and change status to reviewed
			newDocNumber := "A0100000-" + existingInvoice.DocNumber[10:]
			invoiceToUpdate := struct {
				Id        string `json:"Id"`
				SyncToken string `json:"SyncToken"`
				Sparse    bool   `json:"sparse"`
				DocNumber string `json:"DocNumber"`
				// DueDate   string `json:"DueDate"`
			}{
				Id:        invoiceId,
				SyncToken: existingInvoice.SyncToken,
				Sparse:    true,
				DocNumber: newDocNumber,
				// DueDate:   newDueDate.Format("2006-01-02"),
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
			params := &twApi.CreateMessageParams{}
			params.SetBody(fmt.Sprintf("Order %s has been approved and is beginning preparation! For more details please visit: https://ordrport.com/franchisee/invoices", invoiceId))
			// Fuck it we hard code the phone number (for now)
			params.SetFrom("+16478009984")
			params.SetTo("+15062329415")

			twResp, err := twc.Api.CreateMessage(params)
			if err != nil {
				log.Printf("Could not send SMS message %s", err)
			} else {
				if twResp.Body != nil {
					log.Println(*twResp.Body)
				} else {
					log.Println(twResp.Body)
				}
			}

		} else if req.Status == "V" {
			newDocNumber := "A0010000" + existingInvoice.DocNumber[10:]
			invoiceToUpdate := struct {
				Id        string `json:"Id"`
				SyncToken string `json:"SyncToken"`
				Sparse    bool   `json:"sparse"`
				DocNumber string `json:"DocNumber"`
			}{
				Id:        invoiceId,
				SyncToken: existingInvoice.SyncToken,
				Sparse:    true,
				DocNumber: newDocNumber,
			}
			log.Println(invoiceToUpdate)
			updatedInvoice, err := qbc.UpdateInvoice(claims.QBCompanyID, &invoiceToUpdate, existingInvoice.SyncToken)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not update invoice", http.StatusInternalServerError)
				return
			}
			log.Println(updatedInvoice)
			// void invoice
			syncTokenInt, err := strconv.Atoi(existingInvoice.SyncToken)
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not convert SyncToken to int", http.StatusInternalServerError)
				return
			}
			err = qbc.VoidInvoice(claims.QBCompanyID, invoiceId, strconv.Itoa(syncTokenInt+1))
			if err != nil {
				log.Println(err)
				http.Error(w, "Could not void invoice", http.StatusInternalServerError)
				return
			}
		} else if req.Status == "Z" {
			if len(existingInvoice.DocNumber) < 8 || existingInvoice.DocNumber[0:8] != "A0100000" {
				http.Error(w, "Invoice is not in pending state. Verify that invoice has not been already been reviewed and was created from ordrport.", http.StatusBadRequest)
				return
			}
			// slice old doc number and change status to reviewed
			newDocNumber := "A0001000-" + existingInvoice.DocNumber[10:]
			invoiceToUpdate := struct {
				Id        string `json:"Id"`
				SyncToken string `json:"SyncToken"`
				Sparse    bool   `json:"sparse"`
				DocNumber string `json:"DocNumber"`
			}{
				Id:        invoiceId,
				SyncToken: existingInvoice.SyncToken,
				Sparse:    true,
				DocNumber: newDocNumber,
			}
			// TODO: Figure out why I'm getting null point on requests using INvoice struct
			// For now just going to initialize my own
			_, err = qbc.UpdateInvoice(claims.QBCompanyID, invoiceToUpdate, existingInvoice.SyncToken)
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
			http.Error(w, "Invalid status", http.StatusBadRequest)
			return
		}
		// respond with 200
		resp := response{Success: true}
		encode(w, r, http.StatusOK, resp)
	}
}

func ListQBInvoices(qbc *qb.Client) http.HandlerFunc {
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
		orderBy := r.URL.Query().Get("order_by")
		pageSize := r.URL.Query().Get("page_size")
		pageToken := r.URL.Query().Get("page_token")
		id := r.URL.Query().Get("id")
		statuses := r.URL.Query().Get("statuses")
		if orderBy == "" {
			orderBy = "DocNumber ASC"
		}
		if pageSize == "" {
			pageSize = "10"
		}
		if pageToken == "" {
			pageToken = "1"
		}
		if statuses == "" {
			statuses = "PRVZ"
		}
		// Because quickbooks doesn't allow to query invoices by name directly, first get ids of customers based on query, and then get invoices for those customers
		if !claims.IsFranchiser {
			id = claims.QBCustomerID
		}

		invoices, err := qbc.QueryInvoices(claims.QBCompanyID, orderBy, pageSize, pageToken, statuses, id)
		if err != nil {
			log.Println(err)
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
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}
		// If franchisee is calling, they can get only get information about themselves
		if !claims.IsFranchiser && customerId != claims.QBCustomerID {
			log.Println("Customer ID")
			http.Error(w, "No Access", http.StatusServiceUnavailable)
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
			http.Error(w, "Could not check if firebase user linked to qb customer", http.StatusInternalServerError)
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
			http.Error(w, "Could not decrypt QB token", http.StatusUnauthorized)
			return
		}
		qbc.SetClient(qb.BearerToken{AccessToken: string(token)})
		// Get query params
		invoiceId := r.PathValue("id")
		if invoiceId == "" {
			http.Error(w, "No id in url", http.StatusBadRequest)
			return
		}
		// Get Invoice from QB
		invoice, err := qbc.FindInvoiceById(claims.QBCompanyID, invoiceId)
		if err != nil {
			http.Error(w, "Could not get invoice", http.StatusInternalServerError)
			return
		}
		// If franchisee is calling, they can get only get information about themselves
		if !claims.IsFranchiser && invoice.CustomerRef.Value != claims.QBCustomerID {
			http.Error(w, "No Access", http.StatusServiceUnavailable)
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
			log.Println(err)
			http.Error(w, "Could not get PDF", http.StatusInternalServerError)
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

func qbToE164Phone(phone string) (string, error) {
	// Remove all non-digit characters
	reg := regexp.MustCompile(`[^0-9]`)
	cleaned := reg.ReplaceAllString(phone, "")

	// Validate length (10 digits for US numbers)
	if len(cleaned) != 10 {
		return "", fmt.Errorf("invalid phone number length: got %d digits, want 10", len(cleaned))
	}

	return "+1" + cleaned, nil
}

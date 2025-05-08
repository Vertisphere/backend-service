package net

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/Vertisphere/backend-service/internal/domain"
	"github.com/Vertisphere/backend-service/internal/storage"
	"github.com/google/uuid"
	"gopkg.in/square/go-jose.v2"
)

func ShowClaims(ctx context.Context) http.HandlerFunc {
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

func LoginQuickbooks(ctx context.Context, fbc *fb.Client, qbc *qb.Client, a *auth.Client, s *storage.SQLStorage) http.HandlerFunc {
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
			logErr(ctx, err, uuid.New().String(), "Invalid request payload", http.StatusBadRequest)
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
			var userFirebaseID string
			createdUserResp, err := fbc.SignUp(userInfo.Email, encodedRealmID, phoneNumber)
			userFirebaseID = createdUserResp.LocalId
			if err != nil {
				// TODO rollback transaction and delete firebase uesr
				log.Println(err.Error())
				if err.Error() == "email already exists" {
					// http.Error(w, "Email already exists", http.StatusBadRequest)
					// Get Firebase id from email
					user, err := a.GetUserByEmail(ctx, userInfo.Email)
					userFirebaseID = user.UID
					if err != nil {
						http.Error(w, "User with Email exists and Could not get user by email", http.StatusInternalServerError)
					}
				} else {
					http.Error(w, "Could not create user", http.StatusInternalServerError)
					return
				}
			}
			err = s.SetCompanyFirebaseID(req.RealmID, userFirebaseID)
			if err != nil {
				// TODO rollback transaction and delete firebase user
				log.Println(err.Error())
				http.Error(w, "Could not link firebase ID of new user to company", http.StatusInternalServerError)
				return
			}
			firebaseID = userFirebaseID
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
		customTokenInternal, err := a.CustomTokenWithClaims(ctx, firebaseID, domain.ClaimsToMap(customClaims))
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

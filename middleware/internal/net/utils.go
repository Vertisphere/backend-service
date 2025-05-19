package net

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"time"

	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/Vertisphere/backend-service/internal/storage"
	"github.com/rs/zerolog/log"
	"gopkg.in/square/go-jose.v2"
)

func logHttpError(err error, msg string, statusCode int, w *http.ResponseWriter) {
	log.Error().Err(err).Msg(msg)
	http.Error(*w, msg, statusCode)
}

func verifyRequest(r *http.Request, w *http.ResponseWriter) bool {
	if r.Method != http.MethodPost {
		logHttpError(nil, "Method not allowed", http.StatusMethodNotAllowed, w)
		return false
	}
	if r.Header.Get("Content-Type") != "application/json" {
		logHttpError(nil, "Unsupported Media Type", http.StatusUnsupportedMediaType, w)
		return false
	}
	return true
}

func getBearerTokenFromDB(s *storage.SQLStorage, qbc *qb.Client, realmID string) (*qb.BearerToken, error) {
	dbCompany, err := s.GetCompany(realmID)
	if err != nil {
		log.Error().Err(err).Msg("Could not get company from DB; may not exist")
		return &qb.BearerToken{}, err
	}

	if !dbCompany.QBBearerTokenExpiry.Before(time.Now()) {
		return &qb.BearerToken{
			AccessToken:  dbCompany.QBBearerToken,
			RefreshToken: dbCompany.QBRefreshToken,
		}, nil
	}

	bearerToken, err := qbc.RefreshToken(dbCompany.QBRefreshToken)
	if err != nil {
		log.Error().Err(err).Msg("Could not make refresh token call to QB")
		return &qb.BearerToken{}, err
	}

	err = s.UpdateTokenForCompany(realmID, bearerToken.AccessToken, bearerToken.ExpiresIn, bearerToken.RefreshToken, bearerToken.XRefreshTokenExpiresIn)
	if err != nil {
		log.Error().Err(err).Msg("Could insert updated refresh token into company in DB")
		return &qb.BearerToken{}, err
	}
	return bearerToken, nil
}

func qbToE164Phone(phone string) string {
	// Remove all non-digit characters
	reg := regexp.MustCompile(`[^0-9]`)
	cleaned := reg.ReplaceAllString(phone, "")

	// Validate length (10 digits for US numbers)
	if len(cleaned) == 10 {
		return "+1" + cleaned
	}
	if len(cleaned) == 11 && cleaned[0] == '1' {
		return "+" + cleaned
	}
	log.Error().Msgf("Couldn't convert qb phone number to E164 %s", phone)
	return ""
}

func encryptToken(token string) (string, error) {
	encKey := config.LoadConfigs().JWEKey
	rawKey, err := base64.StdEncoding.DecodeString(encKey)
	if err != nil {
		return "", err
	}
	encrypter, err := jose.NewEncrypter(jose.A256GCM, jose.Recipient{Algorithm: jose.DIRECT, Key: rawKey}, nil)
	if err != nil {
		return "", err
	}
	object, err := encrypter.Encrypt([]byte(token))
	if err != nil {
		return "", err
	}
	encryptedToken, err := object.CompactSerialize()
	if err != nil {
		return "", err
	}
	return encryptedToken, nil
}

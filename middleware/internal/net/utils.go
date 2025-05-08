package net

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/Vertisphere/backend-service/internal/config"
	"github.com/rs/zerolog"
	"gopkg.in/square/go-jose.v2"
)

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
	if len(cleaned) == 10 {
		return "+1" + cleaned, nil
	}
	if len(cleaned) == 11 && cleaned[0] == '1' {
		return "+" + cleaned, nil
	}

	return "", fmt.Errorf("invalid phone number length: got %d digits, want 10", len(cleaned))
}

// Should use cloud run log levels
// https://cloud.google.com/run/docs/configuring-logging#log_levels
// We also want to print traces, timestamp, and context
func logErr(logger zerolog.Logger, err error, traceID string, message string, statusCode int) {
	if err != nil {
		statusText := fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode))
		logger.Error().
			Err(err).
			Str("traceID", traceID). // Replace with actual trace ID if available
			Str("status", statusText).
			Msg(message)
	}
}

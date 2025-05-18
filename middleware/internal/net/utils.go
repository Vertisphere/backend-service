package net

import (
	"net/http"

	"github.com/rs/zerolog/log"
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

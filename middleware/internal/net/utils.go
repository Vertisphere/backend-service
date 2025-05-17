package net

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

func logHttpError(err error, msg string, statusCode int, w *http.ResponseWriter) {
	log.Error().Err(err)
	http.Error(*w, msg, statusCode)
}

package net

import (
	"context"
	"net/http"

	"firebase.google.com/go/auth"
	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/storage"
	"github.com/twilio/twilio-go"
)

func NewServer(
	ctx context.Context,
	auth *auth.Client,
	store *storage.SQLStorage,
	firebaseClient *fb.Client,
	quickbooksClient *qb.Client,
	twilioClient *twilio.RestClient,

) http.Handler {
	mux := http.NewServeMux()
	addRoutes(
		ctx,
		mux,
		auth,
		store,
		firebaseClient,
		quickbooksClient,
		twilioClient,
	)
	var handler http.Handler = mux
	// The later the middleware is added the earlier it is executed
	handler = verifyToken(auth, handler)
	handler = corsMiddleware(handler)
	return handler
}

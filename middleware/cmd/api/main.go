package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	firebase "firebase.google.com/go"

	"github.com/twilio/twilio-go"

	fb "github.com/Vertisphere/backend-service/external/firebase"
	qb "github.com/Vertisphere/backend-service/external/quickbooks"
	"github.com/Vertisphere/backend-service/internal/config"
	mynet "github.com/Vertisphere/backend-service/internal/net"
	"github.com/Vertisphere/backend-service/internal/storage"

	"github.com/rs/zerolog"
)

func main() {
	ctx := context.Background()
	err := config.LoadEnv()
	if err != nil {
		log.Fatalf("error loading env: %v\n", err)
	}
	c := config.LoadConfigs()

	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	auth, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error initializing google admin auth client: %v\n", err)
	}
	// read from envar to Use cache or not to use cache
	var store storage.SQLStorage
	if c.UseCache == "true" {
		log.Println("Using cache")
		log.Fatalf("Cache not implemented")
	} else {
		log.Println("Not using cache")
		log.Printf("User: %s, Host: %s, Name: %s", c.DB.User, c.DB.Host, c.DB.Name)
		if c.Env == "prod" {
			log.Println("Using prod db")
			if err := store.Init(c.DB.User, c.DB.Password, c.DB.Host, c.DB.Name, true); err != nil {
				log.Fatalf("error initializing storage: %v\n", err)
			}
		} else {
			if err := store.Init(c.DB.User, c.DB.Password, c.DB.Host, c.DB.Name, true); err != nil {
				log.Fatalf("error initializing storage: %v\n", err)
			}
		}
	}
	defer store.Close()

	// firebase client
	firebaseClient, err := fb.NewClient(c.Firebase.APIKey)
	if err != nil {
		log.Fatalf("error initializing firebase client: %v\n", err)
	}

	// quickbooksClient
	quickbooksClient, err := qb.NewClient(c.Quickbooks.ClientID, c.Quickbooks.ClientSecret, c.Quickbooks.RedirectURI, c.Quickbooks.IsProduction, c.Quickbooks.MinorVersion)
	if err != nil {
		log.Fatalf("error initializing quickbooks client: %v\n", err)
	}

	// I guess twilio doesn't implement client initialization error handling? I guess you're just supposed to find out during runtime if you fucked up the initialization..
	twilioClient := twilio.NewRestClient()

	// init zerolog logger
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	ctx = context.WithValue(ctx, "logger", logger)

	srv := mynet.NewServer(
		ctx,
		auth,
		&store,
		firebaseClient,
		quickbooksClient,
		twilioClient,
	)

	httpServer := &http.Server{
		Addr:    ":" + c.Port,
		Handler: srv,
	}

	go func() {
		log.Printf("listening on %s\n", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx := context.Background()
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "error shutting down http server: %s\n", err)
		}
	}()
	wg.Wait()
	return
}

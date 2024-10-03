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

	"github.com/Vertisphere/backend-service/internal/config"
	mynet "github.com/Vertisphere/backend-service/internal/net"
	"github.com/Vertisphere/backend-service/internal/storage"
)

func main() {
	ctx := context.Background()

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
		if err := store.Init(c.DB.User, c.DB.Password, c.DB.Host, c.DB.Name); err != nil {
			log.Fatalf("error initializing storage: %v\n", err)
		}
	}
	defer store.Close()

	srv := mynet.NewServer(
		ctx,
		auth,
		&store,
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

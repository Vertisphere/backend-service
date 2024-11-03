package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func LoadEnv() error {
	env := os.Getenv("ENV")
	if env == "prod" || env == "stage" {
		return nil
	}
	log.Println("loading .env file")

	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("load env: %w", err)
	}
	return nil
}

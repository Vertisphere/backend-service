package firebase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/martian/v3/log"
)

// Client is your handle to the Firebase API.
type Client struct {
	apiKey string
}

// NewClient creates a new Firebase client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
	}
}

// CreateUser creates a new user in Firebase.
func (c *Client) CreateUser(email string, password string) error {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signUp?key=" + c.apiKey
	params := map[string]string{
		"email":             email,
		"password":          password,
		"returnSecureToken": "true",
	}

	body, err := json.Marshal(params)
	if err != nil {
		log.Errorf("failed to marshal body: %v", err)
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("firebase request failed: %v", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non 200 response code from Firebase %s", resp.Status)
	}
	return nil
}

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
func NewClient(apiKey string) (*Client, error) {
	return &Client{
		apiKey: apiKey,
	}, nil
}

// CreateUser creates a new user in Firebase.
func (c *Client) SignUp(email string, password string, phone string) (CreateUserResponse, error) {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signUp?key=" + c.apiKey
	// TODO: phoneNumber doesn't work right now
	params := map[string]string{
		"email":             email,
		"password":          password,
		"phoneNumber":       phone,
		"returnSecureToken": "true",
	}

	body, err := json.Marshal(params)
	if err != nil {
		log.Errorf("failed to marshal body: %v", err)
		return CreateUserResponse{}, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("firebase request failed: %v", err)
		return CreateUserResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// read error response from firebase and log
		var errorResponse map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			log.Errorf("failed to decode error response from firebase: %v", err)
		} else {
			log.Errorf("firebase error response: %v", errorResponse)
			log.Errorf("firebase error message: %v", errorResponse["error"])
			errMap, _ := errorResponse["error"].(map[string]interface{})
			log.Errorf("firebase error messagesd: %v", errMap)
			if errMap, ok := errorResponse["error"].(map[string]interface{}); ok {
				log.Errorf("%v", errMap)
				log.Errorf("%v", errMap["message"])
			}
			if errMap, ok := errorResponse["error"].(map[string]interface{}); ok && errMap["message"] == "EMAIL_EXISTS" {
				return CreateUserResponse{}, fmt.Errorf("email already exists")
			}
		}
		return CreateUserResponse{}, fmt.Errorf("non 200 response code from Firebase %s", resp.Status)
	}

	var resData CreateUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return CreateUserResponse{}, err
	}
	return resData, nil
}

func (c *Client) SignInWithCustomToken(customTokenInternal string) (SignInWithCustomTokenResponse, error) {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithCustomToken?key=" + c.apiKey
	params := map[string]string{
		"token":             customTokenInternal,
		"returnSecureToken": "true",
	}

	body, err := json.Marshal(params)
	if err != nil {
		log.Errorf("failed to marshal body: %v", err)
		return SignInWithCustomTokenResponse{}, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("firebase request failed: %v", err)
		return SignInWithCustomTokenResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SignInWithCustomTokenResponse{}, fmt.Errorf("non 200 response code from Firebase %s", resp.Status)
	}

	var resData SignInWithCustomTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return SignInWithCustomTokenResponse{}, err
	}
	return resData, nil
}

// CreateUser creates a new user in Firebase.
func (c *Client) SignInWithPassword(email string, password string) (SignInWithPasswordResponse, error) {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + c.apiKey
	params := map[string]string{
		"email":             email,
		"password":          password,
		"returnSecureToken": "true",
	}

	body, err := json.Marshal(params)
	if err != nil {
		log.Errorf("failed to marshal body: %v", err)
		return SignInWithPasswordResponse{}, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Errorf("firebase request failed: %v", err)
		return SignInWithPasswordResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SignInWithPasswordResponse{}, fmt.Errorf("non 200 response code from Firebase %s", resp.Status)
	}

	var resData SignInWithPasswordResponse
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return SignInWithPasswordResponse{}, err
	}
	return resData, nil
}

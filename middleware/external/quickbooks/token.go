package quickbooks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

type BearerToken struct {
	RefreshToken           string `json:"refresh_token"`
	AccessToken            string `json:"access_token"`
	TokenType              string `json:"token_type"`
	IdToken                string `json:"id_token"`
	ExpiresIn              int64  `json:"expires_in"`
	XRefreshTokenExpiresIn int64  `json:"x_refresh_token_expires_in"`
}

// RefreshToken
// Call the refresh endpoint to generate new tokens
func (c *Client) RefreshToken(refreshToken string) (*BearerToken, error) {
	urlValues := url.Values{}
	urlValues.Set("grant_type", "refresh_token")
	urlValues.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", c.discoveryAPI.TokenEndpoint, bytes.NewBufferString(urlValues.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(c))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(body))
	}

	bearerTokenResponse, err := getBearerTokenResponse(body)
	c.Client = getHttpClient(bearerTokenResponse)

	return bearerTokenResponse, err
}

// RetrieveBearerToken
// Method to retrieve access token (bearer token).
// This method can only be called once
func (c *Client) RetrieveBearerToken(authorizationCode string) (*BearerToken, error) {
	urlValues := url.Values{}
	urlValues.Set("grant_type", "authorization_code")
	urlValues.Set("code", authorizationCode)
	urlValues.Set("redirect_uri", c.redirectURI)
	// urlValues.Set("client_id", c.clientID)
	// urlValues.Set("client_secret", c.clientSecret)

	log.Println("urlValues", urlValues)
	req, err := http.NewRequest("POST", c.discoveryAPI.TokenEndpoint, bytes.NewBufferString(urlValues.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(c))

	// Use http.DefaultClient instead of creating a new client
	// Log the request in a curl-like format
	// curlCommand := "curl -X POST " + c.discoveryAPI.TokenEndpoint +
	// 	" -H 'Accept: */*'" +
	// 	" -H 'Content-Type: application/x-www-form-urlencoded'" +
	// 	" -d '" + urlValues.Encode() + "'"
	// log.Println("Executing request:", curlCommand)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(body))
	}

	return getBearerTokenResponse(body)
}

// RevokeToken
// Call the revoke endpoint to revoke tokens
func (c *Client) RevokeToken(refreshToken string) error {
	client := &http.Client{}
	urlValues := url.Values{}
	urlValues.Add("token", refreshToken)

	req, err := http.NewRequest("POST", c.discoveryAPI.RevocationEndpoint, bytes.NewBufferString(urlValues.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Authorization", "Basic "+basicAuth(c))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(string(body))
	}

	c.Client = nil

	return nil
}

func basicAuth(c *Client) string {
	return base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.clientSecret))
}

func getBearerTokenResponse(body []byte) (*BearerToken, error) {
	token := BearerToken{}

	if err := json.Unmarshal(body, &token); err != nil {
		return nil, errors.New(string(body))
	}

	return &token, nil
}

func getHttpClient(bearerToken *BearerToken) *http.Client {
	ctx := context.Background()
	token := oauth2.Token{
		AccessToken: bearerToken.AccessToken,
		TokenType:   "Bearer",
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(&token))
}

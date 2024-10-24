package quickbooks

import "net/http"

// Client is your handle to the QuickBooks API.
type Client struct {
	// Get this from oauth2.NewClient().
	Client       *http.Client
	clientID     string
	clientSecret string
	redirectURI  string
	isProduction bool
}

// NewClient initializes a new QuickBooks client for interacting with their Online API
func NewClient(clientID, clientSecret, redirectURI string, isProduction bool) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		isProduction: isProduction,
	}
}

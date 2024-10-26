package quickbooks

import (
	"encoding/json"
	"errors"
	"net/http"
)

type UserInfo struct {
	Sub                 string `json:"sub"`
	GivenName           string `json:"givenName"`
	FamilyName          string `json:"familyName"`
	Email               string `json:"email"`
	EmailVerified       bool   `json:"emailVerified"`
	PhoneNumber         string `json:"phoneNumber"`
	PhoneNumberVerified bool   `json:"phoneNumberVerified"`
}

func (c *Client) GetUserInfo() (*UserInfo, error) {
	// Prepare the request
	url := c.discoveryAPI.UserinfoEndpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Send the request
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch user info: " + resp.Status)
	}

	// Decode the JSON response into UserInfo
	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

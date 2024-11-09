package quickbooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// Client is your handle to the QuickBooks API.
type Client struct {
	// Get this from oauth2.NewClient().
	Client       *http.Client
	clientID     string
	clientSecret string
	redirectURI  string
	isProduction bool
	discoveryAPI *DiscoveryAPI
	endpoint     EndpointUrl
	throttled    bool
	minorVersion string
}

// NewClient initializes a new QuickBooks client for interacting with their Online API
func NewClient(clientID, clientSecret, redirectURI string, isProduction bool, minorVersion string) (*Client, error) {
	client := Client{
		Client:       nil,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		isProduction: isProduction,
		throttled:    false,
		minorVersion: minorVersion,
	}
	var err error
	if isProduction {
		client.discoveryAPI, err = CallDiscoveryAPI(DiscoveryProductionEndpoint)

		client.endpoint = ProductionEndpoint
	} else {
		client.discoveryAPI, err = CallDiscoveryAPI(DiscoverySandboxEndpoint)
		client.endpoint = SandboxEndpoint
	}
	if err != nil {
		return nil, err
	}
	log.Println(client.endpoint)
	return &client, nil
}

func (c *Client) SetClient(bearerToken BearerToken) {
	ctx := context.Background()
	token := oauth2.Token{
		AccessToken: bearerToken.AccessToken,
		TokenType:   "Bearer",
	}
	c.Client = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&token))
}

// FindAuthorizationUrl compiles the authorization url from the discovery api's auth endpoint.
//
// Example: qbClient.FindAuthorizationUrl("com.intuit.quickbooks.accounting", "security_token", "https://developer.intuit.com/v2/OAuth2Playground/RedirectUrl")
//
// You can find live examples from https://developer.intuit.com/app/developer/playground
func (c *Client) FindAuthorizationUrl(scope string, state string, redirectUri string) (string, error) {
	var authorizationUrl *url.URL

	authorizationUrl, err := url.Parse(c.discoveryAPI.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse auth endpoint: %v", err)
	}

	urlValues := url.Values{}
	urlValues.Add("client_id", c.clientID)
	urlValues.Add("response_type", "code")
	urlValues.Add("scope", scope)
	urlValues.Add("redirect_uri", redirectUri)
	urlValues.Add("state", state)
	authorizationUrl.RawQuery = urlValues.Encode()

	return authorizationUrl.String(), nil
}

func (c *Client) req(realmID string, method string, endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
	// TODO: possibly just wait until c.throttled is false, and continue the request?
	if c.throttled {
		return errors.New("waiting for rate limit")
	}
	var err error
	companyEndpoint, err := url.Parse(string(c.endpoint) + "/v3/company/" + realmID + "/")
	if err != nil {
		return errors.New("failed to parse API endpoint")
	}
	endpointUrl := companyEndpoint
	endpointUrl.Path += endpoint
	urlValues := url.Values{}

	if len(queryParameters) > 0 {
		for param, value := range queryParameters {
			urlValues.Add(param, value)
		}
	}

	urlValues.Set("minorversion", c.minorVersion)
	urlValues.Encode()
	endpointUrl.RawQuery = urlValues.Encode()

	var marshalledJson []byte

	if payloadData != nil {
		marshalledJson, err = json.Marshal(payloadData)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %v", err)
		}
	}

	log.Println(string(marshalledJson))
	req, err := http.NewRequest(method, endpointUrl.String(), bytes.NewBuffer(marshalledJson))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	log.Println(req)
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusTooManyRequests:
		c.throttled = true
		go func(c *Client) {
			time.Sleep(1 * time.Minute)
			c.throttled = false
		}(c)
	default:
		return parseFailure(resp)
	}

	if responseObject != nil {
		if err = json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
			return fmt.Errorf("failed to unmarshal response into object: %v", err)
		}
	}

	return nil
}

func (c *Client) get(realmID string, endpoint string, responseObject interface{}, queryParameters map[string]string) error {
	return c.req(realmID, "GET", endpoint, nil, responseObject, queryParameters)
}

func (c *Client) post(realmID string, endpoint string, payloadData interface{}, responseObject interface{}, queryParameters map[string]string) error {
	return c.req(realmID, "POST", endpoint, payloadData, responseObject, queryParameters)
}

// query makes the specified QBO `query` and unmarshals the result into `responseObject`
func (c *Client) query(realmID string, query string, responseObject interface{}) error {
	return c.get(realmID, "query", responseObject, map[string]string{"query": query})
}

// TODO add errors for 401 when qb access token expires
// 401 {"warnings":null,"intuitObject":null,"fault":{"error":[{"message":"message=AuthenticationFailed; errorCode=003200; statusCode=401","detail":"Token expired: AB01730024136mqEofYN9fldzIcxH6lsAcqOYIbafE5z6ZdjPc","code":"3200","element":null}],"type":"AUTHENTICATION"},"report":null,"queryResponse":null,"batchItemResponse":[],"attachableResponse":[],"syncErrorResponse":null,"requestId":null,"time":1730024394085,"status":null,"cdcresponse":[]}

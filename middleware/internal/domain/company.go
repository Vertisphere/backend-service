package domain

import "time"

type Company struct {
	QBCompanyID          string    `json:"company_id" db:"qb_company_id,omitempty"`
	QBAuthCode           string    `json:"auth_code" db:"qb_auth_code,omitempty"`
	QBBearerToken        string    `json:"bearer_token" db:"qb_bearer_token,omitempty"`
	QBBearerTokenExpiry  time.Time `json:"bearer_token_expiry" db:"qb_bearer_token_expiry,omitempty"`
	QBRefreshToken       string    `json:"refresh_token" db:"qb_refresh_token,omitempty"`
	QBRefreshTokenExpiry time.Time `json:"refresh_token_expiry" db:"qb_refresh_token_expiry,omitempty"`
	FirebaseID           string    `json:"firebase_id" db:"firebase_id,omitempty"`
}

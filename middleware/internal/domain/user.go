package domain

import "time"

type User struct {
	UserID       int       `json:"user_id,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	FranchiseID  int       `json:"franchise_id,omitempty"`
	FranchiseeID int       `json:"franchisee_id,omitempty"`
	Role         int       `json:"role,omitempty"`
	Name         string    `json:"name,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

package domain

import "time"

type User struct {
	UserID       int       `json:"user_id"`
	AccountID    string    `json:"account_id"`
	FranchiseID  int       `json:"franchise_id"`
	FranchiseeID int       `json:"franchisee_id"`
	Role         int       `json:"role"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

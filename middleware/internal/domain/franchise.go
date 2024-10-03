// CREATE TABLE franchise (
//
//	franchise_id SERIAL PRIMARY KEY,
//	franchise_name VARCHAR(100) NOT NULL,
//	headquarters_address VARCHAR(255),
//	phone_number VARCHAR(20),
//	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
//
// );
package domain

import "time"

type Franchise struct {
	FranchiseID         int       `json:"franchise_id,omitempty" db:"franchise_id"`
	FranchiseName       string    `json:"franchise_name" db:"franchise_name"`
	HeadquartersAddress string    `json:"headquarters_address" db:"headquarters_address"`
	PhoneNumber         string    `json:"phone_number" db:"phone_number"`
	CreatedAt           time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

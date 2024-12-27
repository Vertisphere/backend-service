package domain

import (
	"time"

	qb "github.com/Vertisphere/backend-service/external/quickbooks"
)

type DBCustomer struct {
	QBCustomerID string    `json:"qb_customer_id" db:"qb_customer_id"`
	QBCompanyID  string    `json:"qb_company_id" db:"qb_company_id"`
	FirebaseID   string    `json:"firebase_id" db:"firebase_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Customer struct {
	qb.Customer
	DBCustomer
}

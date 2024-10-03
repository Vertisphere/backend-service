package domain

import "time"

type Product struct {
	ID         int
	MerchantID int
	Status     int
	Price      float64
	CreatedAt  time.Time
}

package domain

import "time"

// CREATE TABLE product (
//     product_id SERIAL PRIMARY KEY,
//     franchise_id INT REFERENCES franchise(franchise_id),
//     product_name VARCHAR(100) NOT NULL,
//     description TEXT,
//     price DECIMAL(10, 2) NOT NULL,
//     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//     updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
// );

type Product struct {
	ID            int     `json:"product_id" db:"product_id"`
	FranchiseID   int     `json:"franchise_id" db:"franchise_id"`
	ProductName   string  `json:"product_name" db:"product_name"`
	Description   string  `json:"description" db:"description"`
	Price         float64 `json:"price" db:"price"`
	ProductStatus int     `json:"product_status" db:"product_status"`

	CreatedAt time.Time `json:"createed_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

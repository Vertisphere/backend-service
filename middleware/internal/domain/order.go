package domain

// type Order {
// 	ID int

// }

type OrderRequest struct {
	Products `json:"products"`
}

type Products []struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

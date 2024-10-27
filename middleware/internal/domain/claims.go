package domain

type Claims struct {
	QBCompanyID   string `json:"qb_company_id"`
	QBCustomerID  string `json:"qb_customer_id"` // 0 if franchiser
	QBBearerToken string `json:"qb_bearer_token"`
	IsFranchiser  bool   `json:"is_franchiser"`
	FirebaseID    string `json:"firebase_id"`
}

func ClaimsToMap(claims Claims) map[string]interface{} {
	return map[string]interface{}{
		"qb_company_id":   claims.QBCompanyID,
		"qb_customer_id":  claims.QBCustomerID,
		"qb_bearer_token": claims.QBBearerToken,
		"is_franchiser":   claims.IsFranchiser,
		"firebase_id":     claims.FirebaseID,
	}
}

// // Copyright (c) 2018, Randy Westlund. All rights reserved.
// // This code is under the BSD-2-Clause license.

package quickbooks

// import (
// 	"fmt"
// )

// // Item represents a QuickBooks Item object (a product type).
// // If it's not TaxGroup then it's either TAX or NON. There's no rate so we ignore those or use a default rate?
// // In Canada, I'm pretty sure they're using tax groups so fuck it, this is how we'll get the rates.
// // In US, you pretty just slap on "taxable or not and that's it like wtf"

// type TaxCode struct {
// 	Id               string `json:"Id,omitempty"`
// 	Name             string
// 	Description      string      `json:",omitempty"`
// 	Active           bool        `json:",omitempty"`
// 	TaxGroup         bool        `json:",omitempty"`
// 	Hidden           bool        `json:",omitempty"`
// 	SalesTaxRateList TaxRateList `json:",omitempty"`
// }
// type TaxRateList struct {
// 	TaxRateDetail []struct {
// 		TaxRateRef []struct {
// 			Value string `json:"value,omitempty"`
// 		} `json:"TaxRateRef,omitempty"`
// 	} `json:"TaxRateDetail,omitempty"`
// }

// type TaxRate struct {
// 	Id        string `json:",omitempty"`
// 	RateValue int    `json:",omitempty"`
// }

// // FindItemById returns an item with a given Id.
// func (c *Client) GetTaxCodeToRate(realmID string, id string) (*Item, error) {
// 	var respCodes struct {
// 		QueryResponse struct {
// 			TaxCodes      []TaxCode `json:"TaxCode"`
// 			StartPosition int
// 			MaxResults    int
// 		}
// 	}
// 	var respRates struct {
// 		QueryResponse struct {
// 			TaxRates      []TaxRate `json:"TaxRate"`
// 			StartPosition int
// 			MaxResults    int
// 		}
// 	}

// 	queryCodes := fmt.Sprintf("SELECT * FROM TaxCode WHERE Active=true")
// 	if err := c.query(realmID, queryCodes, &respCodes); err != nil {
// 		return nil, err
// 	}
// 	queryRates := fmt.Sprintf("SELECT * FROM TaxRate")
// 	if err := c.query(realmID, queryRates, &respRates); err != nil {
// 		return nil, err
// 	}

// 	codesToRateRef := make(map[string]string)
// 	for _, taxCode := range respCodes.QueryResponse.TaxCodes {
// 		codesToRateRef[taxCode.Id] = taxCode.SalesTaxRateList.TaxRateDetail[0].TaxRateRef[0].Value
// 	}
// 	for _, taxRate := range respRates.QueryResponse.TaxRates {

// 	}

// Just realized that we can just use the prices and then once the invoice is created, we can just return the total with tax includes from that wtf
// 	return &resp.Item, nil
// }

// // QueryCustomers accepts an SQL query and returns all customers found using it
// func (c *Client) QueryItems(realmID string, orderBy string, pageSize string, pageToken string, searchQuery string) ([]Item, error) {
// 	var resp struct {
// 		QueryResponse struct {
// 			Items         []Item `json:"Item"`
// 			StartPosition int
// 			MaxResults    int
// 		}
// 	}

// 	// ONLY INVENTORY ITEMS FOR NOW
// 	query := fmt.Sprintf("SELECT * FROM Item WHERE Type='Inventory AND Name LIKE '%%%s%%' orderBy %s MAXRESULTS %s STARTPOSITION %s", searchQuery, orderBy, pageSize, pageToken)
// 	if err := c.query(realmID, query, &resp); err != nil {
// 		return nil, err
// 	}

// 	// if resp.QueryResponse.Items == nil {
// 	// 	return nil, errors.New("could not find any items")
// 	// }

// 	return resp.QueryResponse.Items, nil
// }

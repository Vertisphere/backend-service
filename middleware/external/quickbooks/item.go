// Copyright (c) 2018, Randy Westlund. All rights reserved.
// This code is under the BSD-2-Clause license.

package quickbooks

import (
	"encoding/json"
	"fmt"
)

// Item represents a QuickBooks Item object (a product type).
type Item struct {
	Id          string   `json:"Id,omitempty"`
	SyncToken   string   `json:",omitempty"`
	MetaData    MetaData `json:",omitempty"`
	Name        string
	SKU         string `json:"Sku,omitempty"`
	Description string `json:",omitempty"`
	Active      bool   `json:",omitempty"`
	// SubItem
	// ParentRef
	// Level
	// FullyQualifiedName
	Taxable             bool        `json:",omitempty"`
	SalesTaxIncluded    bool        `json:",omitempty"`
	UnitPrice           json.Number `json:",omitempty"`
	Type                string
	IncomeAccountRef    ReferenceType
	ExpenseAccountRef   ReferenceType
	PurchaseDesc        string      `json:",omitempty"`
	PurchaseTaxIncluded bool        `json:",omitempty"`
	PurchaseCost        json.Number `json:",omitempty"`
	AssetAccountRef     ReferenceType
	TrackQtyOnHand      bool `json:",omitempty"`
	// InvStartDate Date
	QtyOnHand          json.Number   `json:",omitempty"`
	SalesTaxCodeRef    ReferenceType `json:",omitempty"`
	PurchaseTaxCodeRef ReferenceType `json:",omitempty"`
}

// FindItemById returns an item with a given Id.
func (c *Client) FindItemById(realmId string, id string) (*Item, error) {
	var resp struct {
		Item Item
		Time Date
	}

	if err := c.get(realmId, "item/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Item, nil
}

// QueryCustomers accepts an SQL query and returns all customers found using it
func (c *Client) QueryItems(realmID string, orderBy string, pageSize string, pageToken string, searchQuery string) ([]Item, error) {
	var resp struct {
		QueryResponse struct {
			Items         []Item `json:"Item"`
			StartPosition int
			MaxResults    int
		}
	}

	// ONLY INVENTORY ITEMS FOR NOW
	query := fmt.Sprintf("SELECT * FROM Item WHERE Type='Inventory' AND Name LIKE '%%%s%%' orderBy %s MAXRESULTS %s STARTPOSITION %s", searchQuery, orderBy, pageSize, pageToken)
	if err := c.query(realmID, query, &resp); err != nil {
		return nil, err
	}

	// if resp.QueryResponse.Items == nil {
	// 	return nil, errors.New("could not find any items")
	// }

	return resp.QueryResponse.Items, nil
}

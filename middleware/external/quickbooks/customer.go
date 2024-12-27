// Copyright (c) 2018, Randy Westlund. All rights reserved.
// This code is under the BSD-2-Clause license.

package quickbooks

import (
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/guregu/null.v4"
)

// Customer represents a QuickBooks Customer object.
type Customer struct {
	Id                 string          `json:",omitempty"`
	SyncToken          string          `json:",omitempty"`
	MetaData           MetaData        `json:",omitempty"`
	Title              string          `json:",omitempty"`
	GivenName          string          `json:",omitempty"`
	MiddleName         string          `json:",omitempty"`
	FamilyName         string          `json:",omitempty"`
	Suffix             string          `json:",omitempty"`
	DisplayName        string          `json:",omitempty"`
	FullyQualifiedName string          `json:",omitempty"`
	CompanyName        string          `json:",omitempty"`
	PrintOnCheckName   string          `json:",omitempty"`
	Active             bool            `json:",omitempty"`
	PrimaryPhone       TelephoneNumber `json:",omitempty"`
	AlternatePhone     TelephoneNumber `json:",omitempty"`
	Mobile             TelephoneNumber `json:",omitempty"`
	Fax                TelephoneNumber `json:",omitempty"`
	CustomerTypeRef    ReferenceType   `json:",omitempty"`
	PrimaryEmailAddr   *EmailAddress   `json:",omitempty"`
	WebAddr            *WebSiteAddress `json:",omitempty"`
	// DefaultTaxCodeRef
	Taxable              *bool            `json:",omitempty"`
	TaxExemptionReasonId *string          `json:",omitempty"`
	BillAddr             *PhysicalAddress `json:",omitempty"`
	ShipAddr             *PhysicalAddress `json:",omitempty"`
	Notes                string           `json:",omitempty"`
	Job                  null.Bool        `json:",omitempty"`
	BillWithParent       bool             `json:",omitempty"`
	ParentRef            ReferenceType    `json:",omitempty"`
	Level                int              `json:",omitempty"`
	// SalesTermRef
	// PaymentMethodRef
	Balance         json.Number `json:",omitempty"`
	OpenBalanceDate Date        `json:",omitempty"`
	BalanceWithJobs json.Number `json:",omitempty"`
	// CurrencyRef
}

// FindCustomerById returns a customer with a given Id.
func (c *Client) GetCustomerById(realmID string, id string) (*Customer, error) {
	var r struct {
		Customer Customer
		Time     Date
	}

	if err := c.get(realmID, "customer/"+id, &r, nil); err != nil {
		return nil, err
	}

	return &r.Customer, nil
}

func (c *Client) GetCustomerIdsByName(realmID string, name string) ([]string, error) {
	var r struct {
		QueryResponse struct {
			Customers []Customer `json:"Customer"`
		}
	}

	query := fmt.Sprintf("SELECT id FROM Customer WHERE DisplayName LIKE '%%%s%%'", name)
	if err := c.query(realmID, query, &r); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(r.QueryResponse.Customers))
	for _, customer := range r.QueryResponse.Customers {
		ids = append(ids, customer.Id)
	}
	return ids, nil
}

// QueryCustomers accepts an SQL query and returns all customers found using it
func (c *Client) QueryCustomers(realmID string, orderBy string, pageSize string, pageToken string, searchQuery string) ([]Customer, error) {
	var resp struct {
		QueryResponse struct {
			Customers     []Customer `json:"Customer"`
			StartPosition int
			MaxResults    int
		}
	}
	query := fmt.Sprintf("SELECT * FROM Customer WHERE %s orderBy %s MAXRESULTS %s STARTPOSITION %s", searchQuery, orderBy, pageSize, pageToken)
	if err := c.query(realmID, query, &resp); err != nil {
		return nil, err
	}

	// if resp.QueryResponse.Customers == nil {
	// 	return nil, errors.New("could not find any customers")
	// }

	return resp.QueryResponse.Customers, nil
}

// UpdateCustomer updates the given Customer on the QuickBooks server,
// returning the resulting Customer object. It's a sparse update, as not all QB
// fields are present in our Customer object.
func (c *Client) UpdateCustomer(realmID string, customer *Customer) (*Customer, error) {
	if customer.Id == "" {
		return nil, errors.New("missing customer id")
	}
	if customer.SyncToken == "" {
		return nil, errors.New("missing customer sync token")
	}

	payload := struct {
		*Customer
		Sparse bool `json:"sparse"`
	}{
		Customer: customer,
		Sparse:   true,
	}

	var customerData struct {
		Customer Customer
		Time     Date
	}

	var err error
	if err = c.post(realmID, "customer", payload, &customerData, nil); err != nil {
		return nil, err
	}

	return &customerData.Customer, nil
}

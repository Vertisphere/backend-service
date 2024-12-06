// Copyright (c) 2018, Randy Westlund. All rights reserved.
// This code is under the BSD-2-Clause license.

package quickbooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

// Invoice represents a QuickBooks Invoice object.
type Invoice struct {
	Id            string        `json:"Id,omitempty"`
	SyncToken     string        `json:",omitempty"`
	MetaData      MetaData      `json:",omitempty"`
	CustomField   []CustomField `json:",omitempty"`
	DocNumber     string        `json:",omitempty"`
	TxnDate       Date          `json:",omitempty"`
	DepartmentRef ReferenceType `json:",omitempty"`
	PrivateNote   string        `json:",omitempty"`
	LinkedTxn     []LinkedTxn   `json:"LinkedTxn"`
	Line          []Line
	TxnTaxDetail  TxnTaxDetail `json:",omitempty"`
	CustomerRef   ReferenceType
	CustomerMemo  MemoRef         `json:",omitempty"`
	BillAddr      PhysicalAddress `json:",omitempty"`
	ShipAddr      PhysicalAddress `json:",omitempty"`
	ClassRef      ReferenceType   `json:",omitempty"`
	SalesTermRef  ReferenceType   `json:",omitempty"`
	DueDate       Date            `json:",omitempty"`
	// GlobalTaxCalculation
	ShipMethodRef                ReferenceType `json:",omitempty"`
	ShipDate                     Date          `json:",omitempty"`
	TrackingNum                  string        `json:",omitempty"`
	TotalAmt                     json.Number   `json:",omitempty"`
	CurrencyRef                  ReferenceType `json:",omitempty"`
	ExchangeRate                 json.Number   `json:",omitempty"`
	HomeAmtTotal                 json.Number   `json:",omitempty"`
	HomeBalance                  json.Number   `json:",omitempty"`
	ApplyTaxAfterDiscount        bool          `json:",omitempty"`
	PrintStatus                  string        `json:",omitempty"`
	EmailStatus                  string        `json:",omitempty"`
	BillEmail                    EmailAddress  `json:",omitempty"`
	BillEmailCC                  EmailAddress  `json:"BillEmailCc,omitempty"`
	BillEmailBCC                 EmailAddress  `json:"BillEmailBcc,omitempty"`
	DeliveryInfo                 *DeliveryInfo `json:",omitempty"`
	Balance                      json.Number   `json:",omitempty"`
	TxnSource                    string        `json:",omitempty"`
	AllowOnlineCreditCardPayment bool          `json:",omitempty"`
	AllowOnlineACHPayment        bool          `json:",omitempty"`
	Deposit                      json.Number   `json:",omitempty"`
	DepositToAccountRef          ReferenceType `json:",omitempty"`
}

type DeliveryInfo struct {
	DeliveryType string
	DeliveryTime Date
}

type LinkedTxn struct {
	TxnID   string `json:"TxnId"`
	TxnType string `json:"TxnType"`
}

type TxnTaxDetail struct {
	TxnTaxCodeRef ReferenceType `json:",omitempty"`
	TotalTax      json.Number   `json:",omitempty"`
	TaxLine       []Line        `json:",omitempty"`
}

type AccountBasedExpenseLineDetail struct {
	AccountRef ReferenceType
	TaxAmount  json.Number `json:",omitempty"`
	// TaxInclusiveAmt json.Number              `json:",omitempty"`
	// ClassRef        ReferenceType `json:",omitempty"`
	// TaxCodeRef      ReferenceType `json:",omitempty"`
	// MarkupInfo MarkupInfo `json:",omitempty"`
	// BillableStatus BillableStatusEnum       `json:",omitempty"`
	// CustomerRef    ReferenceType `json:",omitempty"`
}

type Line struct {
	Id                            string `json:",omitempty"`
	LineNum                       int    `json:",omitempty"`
	Description                   string `json:",omitempty"`
	Amount                        json.Number
	DetailType                    string
	AccountBasedExpenseLineDetail AccountBasedExpenseLineDetail `json:",omitempty"`
	SalesItemLineDetail           SalesItemLineDetail           `json:",omitempty"`
	DiscountLineDetail            DiscountLineDetail            `json:",omitempty"`
	TaxLineDetail                 TaxLineDetail                 `json:",omitempty"`
}

// TaxLineDetail ...
type TaxLineDetail struct {
	PercentBased     bool        `json:",omitempty"`
	NetAmountTaxable json.Number `json:",omitempty"`
	// TaxInclusiveAmount json.Number `json:",omitempty"`
	// OverrideDeltaAmount
	TaxPercent json.Number `json:",omitempty"`
	TaxRateRef ReferenceType
}

// SalesItemLineDetail ...
type SalesItemLineDetail struct {
	ItemRef   ReferenceType `json:",omitempty"`
	ClassRef  ReferenceType `json:",omitempty"`
	UnitPrice json.Number   `json:",omitempty"`
	// MarkupInfo
	Qty             float64       `json:",omitempty"`
	ItemAccountRef  ReferenceType `json:",omitempty"`
	TaxCodeRef      ReferenceType `json:",omitempty"`
	ServiceDate     Date          `json:",omitempty"`
	TaxInclusiveAmt json.Number   `json:",omitempty"`
	DiscountRate    json.Number   `json:",omitempty"`
	DiscountAmt     json.Number   `json:",omitempty"`
}

// DiscountLineDetail ...
type DiscountLineDetail struct {
	PercentBased    bool
	DiscountPercent float32 `json:",omitempty"`
}

// CreateInvoice creates the given Invoice on the QuickBooks server, returning
// the resulting Invoice object.
func (c *Client) CreateInvoice(realmID string, invoice *Invoice) (*Invoice, error) {
	var resp struct {
		Invoice Invoice
		Time    Date
	}

	if err := c.post(realmID, "invoice", invoice, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Invoice, nil
}

// // FindInvoiceById finds the invoice by the given id
func (c *Client) FindInvoiceById(realmID string, id string) (*Invoice, error) {
	var resp struct {
		Invoice Invoice
		Time    Date
	}

	if err := c.get(realmID, "invoice/"+id, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.Invoice, nil
}

// // SendInvoice sends the invoice to the Invoice.BillEmail if emailAddress is left empty
func (c *Client) SendInvoice(realmID string, invoiceId string, emailAddress string) error {
	queryParameters := make(map[string]string)

	var resp struct {
		Invoice Invoice
		Time    Date
	}

	if emailAddress != "" {
		queryParameters["sendTo"] = emailAddress
	}

	return c.post(realmID, "invoice/"+invoiceId+"/send", nil, &resp, queryParameters)
}

// // UpdateInvoice updates the invoice
// Usually you should know that you need to pass synctoken, so I'll leave a parameter to remind that
func (c *Client) UpdateInvoice(realmID string, invoice interface{}, syncToken string) (*Invoice, error) {

	// invoice.SyncToken = syncToken

	// payload := struct {
	// 	DueDate string `json:"DueDate,omitempty"`
	// 	Sparse  bool   `json:"sparse"`
	// }{
	// 	Invoice: invoice,
	// 	DueDate: newDueDate,
	// 	Sparse:  true,
	// }

	var invoiceData struct {
		Invoice Invoice
		Time    Date
	}

	var err error
	if err = c.post(realmID, "invoice", invoice, &invoiceData, nil); err != nil {
		return nil, err
	}

	return &invoiceData.Invoice, err
}

func (c *Client) VoidInvoice(realmID string, invoiceId string, syncToken string) error {
	if invoiceId == "" {
		return errors.New("missing invoice id")
	}
	invoice := &Invoice{
		Id:        invoiceId,
		SyncToken: syncToken,
	}

	return c.post(realmID, "invoice", invoice, nil, map[string]string{"operation": "void"})
}

func (c *Client) QueryInvoices(realmID string, orderBy string, pageSize string, pageToken string, statuses string, id string) (interface{}, error) {
	var resp struct {
		QueryResponse struct {
			// Getting custom struct here since we don't need "all" of the information in the list view
			Invoices []struct {
				Id          string        `json:"Id,omitempty"`
				CustomerRef ReferenceType `json:"CustomerRef,omitempty"`
				DocNumber   string        `json:"DocNumber,omitempty"`
				TxnDate     Date          `json:"TxnDate,omitempty"`
				TotalAmt    json.Number   `json:"TotalAmt,omitempty"`
				Balance     json.Number   `json:"Balance,omitempty"`
			} `json:"Invoice"`
			StartPosition int
			MaxResults    int
		}
	}

	query := "SELECT * FROM Invoice WHERE DocNumber > 'A'"
	if id != "" {
		query += fmt.Sprintf(" AND customerRef = '%s'", id)
	}
	if statuses != "" {
		// Constructs a wildcard mask for searching statuses that we want
		// We get 21 characters for the DocNumber field
		// Format is "APRVZ000-YYMMDDHHMMSS"
		// A is a place holder to indicate that it's from our system
		// P(pending), R(reviewed/approved), V(voided), Z(completed) are the statuses we want to search for: 1 for the current status, 0 if we don't
		// For example, if we want want all statuses except voided we search for ... LIKE 'A%%0%000-%'
		// - is a separator
		// YYMMDDHHMMSS is the timestamp
		// Should guarantee uniqueness for now, might need to change as we scale
		p, r, v, z := "0", "0", "0", "0"
		for i := 0; i < min(len(statuses), 4); i++ {
			switch statuses[i] {
			case 'P':
				p = "%"
			case 'R':
				r = "%"
			case 'V':
				v = "%"
			case 'Z':
				z = "%"
			}
		}
		query += fmt.Sprintf(" AND DocNumber LIKE 'A%s%s%s%s000-%%'", p, v, r, z)
	}

	query += fmt.Sprintf(" ORDER BY %s MAXRESULTS %s STARTPOSITION %s", orderBy, pageSize, pageToken)

	log.Println(query)

	if err := c.query(realmID, query, &resp); err != nil {
		return nil, err
	}

	return resp.QueryResponse.Invoices, nil
}

func (c *Client) GetInvoicePDF(realmID string, invoiceId string) ([]byte, error) {
	if c.throttled {
		return nil, errors.New("waiting for rate limit")
	}
	endpointUrl, err := url.Parse(string(c.endpoint) + "/v3/company/" + realmID + "/")
	if err != nil {
		return nil, errors.New("failed to parse API endpoint")
	}

	endpointUrl.Path += "invoice/" + invoiceId + "/pdf"

	urlValues := url.Values{}
	urlValues.Set("minorversion", c.minorVersion)
	urlValues.Encode()
	endpointUrl.RawQuery = urlValues.Encode()

	var marshalledJson []byte

	req, err := http.NewRequest("GET", endpointUrl.String(), bytes.NewBuffer(marshalledJson))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/pdf")

	log.Println(req)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get pdf")
	}

	// Take "%PDF-1.4\r\n...\r\n%%EOF" object and return it as content-type: application/pdf
	return io.ReadAll(resp.Body)
}

// Copyright (c) 2018, Randy Westlund. All rights reserved.
// This code is under the BSD-2-Clause license.

package quickbooks

// CompanyInfo describes a company account.
type CompanyInfo struct {
	CompanyName string
	LegalName   string
	// CompanyAddr
	// CustomerCommunicationAddr
	// LegalAddr
	// PrimaryPhone
	// CompanyStartDate     Date
	CompanyStartDate     string
	FiscalYearStartMonth string
	Country              string
	// Email
	// WebAddr
	SupportedLanguages string
	// NameValue
	Domain    string
	Id        string
	SyncToken string
	Metadata  MetaData `json:",omitempty"`
}

// FindCompanyInfo returns the QuickBooks CompanyInfo object. This is a good
// test to check whether you're connected.
func (c *Client) FindCompanyInfo(realmID string) (*CompanyInfo, error) {
	var resp struct {
		CompanyInfo CompanyInfo
		Time        Date
	}

	if err := c.get(realmID, "companyinfo/"+realmID, &resp, nil); err != nil {
		return nil, err
	}

	return &resp.CompanyInfo, nil
}

// UpdateCompanyInfo updates the company info
func (c *Client) UpdateCompanyInfo(realmID string, companyInfo *CompanyInfo) (*CompanyInfo, error) {
	existingCompanyInfo, err := c.FindCompanyInfo(realmID)
	if err != nil {
		return nil, err
	}

	companyInfo.Id = existingCompanyInfo.Id
	companyInfo.SyncToken = existingCompanyInfo.SyncToken

	payload := struct {
		*CompanyInfo
		Sparse bool `json:"sparse"`
	}{
		CompanyInfo: companyInfo,
		Sparse:      true,
	}

	var companyInfoData struct {
		CompanyInfo CompanyInfo
		Time        Date
	}

	if err = c.post(realmID, "companyInfo", payload, &companyInfoData, nil); err != nil {
		return nil, err
	}

	return &companyInfoData.CompanyInfo, err
}

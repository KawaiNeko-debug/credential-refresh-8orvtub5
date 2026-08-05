package refreshapi

import "strings"

const (
	GroupStatusCompleted = "completed"
	GroupStatusFailed    = "failed"
)

type JobAccount struct {
	CustomerCode string `json:"customerCode"`
	Phone        string `json:"phone,omitempty"`
	Password     string `json:"password,omitempty"`
}

type GroupAccountsResponse struct {
	JobID             string       `json:"jobId"`
	GroupIndex        int          `json:"groupIndex"`
	BrowsersPerRunner int          `json:"browsersPerRunner"`
	Accounts          []JobAccount `json:"accounts"`
}

type ResultRequest struct {
	Results []AccountResult `json:"results"`
}

type AccountResult struct {
	CustomerCode      string `json:"customerCode"`
	Success           bool   `json:"success"`
	Ticket            string `json:"ticket,omitempty"`
	PrimarySession    string `json:"primarySession,omitempty"`
	GroupSession      string `json:"groupSession,omitempty"`
	MobileAccessToken string `json:"mobileAccessToken,omitempty"`
	CanUseVoucher     *int   `json:"canUseVoucher,omitempty"`
	Message           string `json:"message,omitempty"`
}

type GroupCompleteRequest struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NormalizeCustomerCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func MaskCustomerCode(value string) string {
	code := NormalizeCustomerCode(value)
	if len(code) <= 4 {
		return strings.Repeat("*", len(code))
	}
	return "****" + code[4:]
}

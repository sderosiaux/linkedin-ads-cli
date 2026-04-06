package api

// Money is the LinkedIn money envelope used by budgets, bids and conversion
// values. Amount is a decimal string (e.g. "12.34").
type Money struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

// DateRange holds an epoch-millis open or closed interval. Both bounds are
// optional in the LinkedIn API.
type DateRange struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

// Locale is the language/country pair used by campaigns and lead-gen forms.
type Locale struct {
	Country  string `json:"country"`
	Language string `json:"language"`
}

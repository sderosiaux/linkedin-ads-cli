package urn

import "strings"

// Kind is a LinkedIn URN resource type.
type Kind string

const (
	Account       Kind = "sponsoredAccount"
	CampaignGroup Kind = "sponsoredCampaignGroup"
	Campaign      Kind = "sponsoredCampaign"
	Creative      Kind = "sponsoredCreative"
)

// Wrap returns a full LinkedIn URN for the given kind and id. If id already
// looks like a URN (starts with "urn:li:"), it is returned unchanged.
func Wrap(k Kind, id string) string {
	if strings.HasPrefix(id, "urn:li:") {
		return id
	}
	return "urn:li:" + string(k) + ":" + id
}

// Unwrap returns the bare id portion of a URN. If the input is not a URN, it
// is returned unchanged.
func Unwrap(urn string) string {
	i := strings.LastIndex(urn, ":")
	if i < 0 {
		return urn
	}
	return urn[i+1:]
}

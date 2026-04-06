package urn

import "testing"

func TestWrap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		kind Kind
		id   string
		want string
	}{
		{"account", Account, "123", "urn:li:sponsoredAccount:123"},
		{"campaign-group", CampaignGroup, "456", "urn:li:sponsoredCampaignGroup:456"},
		{"campaign", Campaign, "789", "urn:li:sponsoredCampaign:789"},
		{"creative", Creative, "101", "urn:li:sponsoredCreative:101"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Wrap(tc.kind, tc.id); got != tc.want {
				t.Errorf("Wrap(%v, %q) = %q, want %q", tc.kind, tc.id, got, tc.want)
			}
		})
	}
}

func TestWrapIdempotent(t *testing.T) {
	t.Parallel()
	full := "urn:li:sponsoredCampaign:789"
	if got := Wrap(Campaign, full); got != full {
		t.Errorf("Wrap should be idempotent: %q", got)
	}
}

func TestUnwrap(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"urn:li:sponsoredAccount:123":       "123",
		"urn:li:sponsoredCampaignGroup:456": "456",
		"urn:li:sponsoredCampaign:789":      "789",
		"urn:li:sponsoredCreative:101":      "101",
		"789":                               "789",
		"":                                  "",
	}
	for in, want := range cases {
		if got := Unwrap(in); got != want {
			t.Errorf("Unwrap(%q) = %q, want %q", in, got, want)
		}
	}
}

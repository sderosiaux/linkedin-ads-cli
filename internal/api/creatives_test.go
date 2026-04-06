package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListCreatives(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/12345/creatives" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "q=criteria") {
			t.Errorf("RawQuery missing q=criteria: %q", raw)
		}
		// URN colons inside List() are percent-encoded on the wire.
		if !strings.Contains(raw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
			t.Errorf("RawQuery missing campaigns List: %q", raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             "urn:li:sponsoredCreative:1",
					"status":         "ACTIVE",
					"intendedStatus": "ACTIVE",
					"campaign":       "urn:li:sponsoredCampaign:42",
					"review":         map[string]any{"status": "APPROVED"},
					"createdAt":      1700000000000,
					"lastModifiedAt": 1710000000000,
				},
				{
					"id":             "urn:li:sponsoredCreative:2",
					"status":         "DRAFT",
					"intendedStatus": "DRAFT",
					"campaign":       "urn:li:sponsoredCampaign:42",
				},
			},
			"metadata": map[string]any{},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	creatives, err := ListCreatives(context.Background(), c, "12345", "42", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(creatives) != 2 {
		t.Fatalf("len: %d", len(creatives))
	}
	if creatives[0].ID != "urn:li:sponsoredCreative:1" {
		t.Errorf("id[0]: %q", creatives[0].ID)
	}
	if creatives[0].ReviewStatus() != "APPROVED" {
		t.Errorf("review: %q", creatives[0].ReviewStatus())
	}
}

func TestCreateCreative(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("X-RestLi-Id", "urn:li:sponsoredCreative:99")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	in := &CreateCreativeInput{
		Campaign:       "urn:li:sponsoredCampaign:42",
		IntendedStatus: "ACTIVE",
		Content:        &CreativeContent{Reference: "urn:li:share:12345"},
	}
	id, err := CreateCreative(context.Background(), c, "777", in)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/adAccounts/777/creatives" {
		t.Errorf("path: %q", gotPath)
	}
	if id != "urn:li:sponsoredCreative:99" {
		t.Errorf("id: %q", id)
	}
	if !strings.Contains(string(gotBody), `"reference"`) || !strings.Contains(string(gotBody), `"urn:li:share:12345"`) {
		t.Errorf("body: %s", string(gotBody))
	}
}

func TestCreateInlineCreative_BodyShape(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("X-RestLi-Id", "urn:li:sponsoredCreative:55")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	in := &CreateInlineCreativeInput{
		Campaign:       "42",
		AccountID:      "777",
		OrgID:          "789",
		IntendedStatus: "ACTIVE",
		Commentary:     "Check out our product!",
		ImageURN:       "urn:li:image:XXX",
		ImageTitle:     "Product Shot",
		LandingPageURL: "https://example.com",
		CTALabel:       "LEARN_MORE",
	}
	id, err := CreateInlineCreative(context.Background(), c, "777", in)
	if err != nil {
		t.Fatal(err)
	}
	if id != "urn:li:sponsoredCreative:55" {
		t.Errorf("id: %q", id)
	}
	if !strings.Contains(gotPath, "action=createInline") {
		t.Errorf("path: %q", gotPath)
	}

	// Verify nested body structure
	creative, ok := gotBody["creative"].(map[string]any)
	if !ok {
		t.Fatalf("missing creative key in body: %+v", gotBody)
	}
	if creative["campaign"] != "urn:li:sponsoredCampaign:42" {
		t.Errorf("campaign: %v", creative["campaign"])
	}
	inline, ok := creative["inlineContent"].(map[string]any)
	if !ok {
		t.Fatalf("missing inlineContent: %+v", creative)
	}
	post, ok := inline["post"].(map[string]any)
	if !ok {
		t.Fatalf("missing post: %+v", inline)
	}
	if post["author"] != "urn:li:organization:789" {
		t.Errorf("author: %v", post["author"])
	}
	if post["commentary"] != "Check out our product!" {
		t.Errorf("commentary: %v", post["commentary"])
	}
	if post["contentLandingPage"] != "https://example.com" {
		t.Errorf("contentLandingPage: %v", post["contentLandingPage"])
	}
	if post["contentCallToActionLabel"] != "LEARN_MORE" {
		t.Errorf("contentCallToActionLabel: %v", post["contentCallToActionLabel"])
	}
	adCtx, ok := post["adContext"].(map[string]any)
	if !ok {
		t.Fatalf("missing adContext: %+v", post)
	}
	if adCtx["dscAdAccount"] != "urn:li:sponsoredAccount:777" {
		t.Errorf("dscAdAccount: %v", adCtx["dscAdAccount"])
	}
}

func TestUpdateCreativeStatus(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotRestli string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRestli = r.Header.Get("X-RestLi-Method")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if err := UpdateCreativeStatus(context.Background(), c, "12345", "1", "PAUSED"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: %q", gotMethod)
	}
	// The URN is path-escaped: urn%3Ali%3AsponsoredCreative%3A1
	wantDecoded := "/adAccounts/12345/creatives/urn:li:sponsoredCreative:1"
	if gotPath != wantDecoded {
		t.Errorf("path: %q", gotPath)
	}
	if gotRestli != "PARTIAL_UPDATE" {
		t.Errorf("X-RestLi-Method: %q", gotRestli)
	}
	if !strings.Contains(string(gotBody), `"intendedStatus"`) || !strings.Contains(string(gotBody), `"PAUSED"`) {
		t.Errorf("body: %s", string(gotBody))
	}
}

func TestGetCreative(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL-encoded URN: urn%3Ali%3AsponsoredCreative%3A1
		wantDecoded := "/adAccounts/12345/creatives/urn:li:sponsoredCreative:1"
		if r.URL.Path != wantDecoded {
			t.Errorf("path: decoded=%q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"urn:li:sponsoredCreative:1","status":"ACTIVE","intendedStatus":"ACTIVE","campaign":"urn:li:sponsoredCampaign:42"}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	cr, err := GetCreative(context.Background(), c, "12345", "1")
	if err != nil {
		t.Fatal(err)
	}
	if cr.ID != "urn:li:sponsoredCreative:1" {
		t.Errorf("id: %q", cr.ID)
	}
}

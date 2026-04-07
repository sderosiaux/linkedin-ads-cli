package cmd

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/spf13/cobra"
)

// conversionEventRecord is one row of an offline conversion CSV.
type conversionEventRecord struct {
	Email      string
	OccurredAt time.Time
	Value      string
	Currency   string
	EventID    string
}

func newConversionsTrackCmd() *cobra.Command {
	var (
		email      string
		value      string
		currency   string
		occurredAt string
		eventID    string
	)
	cmd := &cobra.Command{
		Use:   "track <rule-id>",
		Short: "Upload a single offline conversion event via the Conversions API",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			if email == "" {
				return errors.New("--email required")
			}
			t, err := parseConversionTime(occurredAt)
			if err != nil {
				return err
			}
			input, err := buildConversionEvent(args[0], conversionEventRecord{
				Email:      email,
				OccurredAt: t,
				Value:      value,
				Currency:   currency,
				EventID:    eventID,
			})
			if err != nil {
				return err
			}
			summary := "POST /conversionEvents"
			return executeWrite(cmd, summary, input, func() error {
				if _, err := api.PostConversionEvent(cmd.Context(), c, input); err != nil {
					return decorateConversionsAPIErr(err)
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "✓ Conversion event uploaded")
				return err
			})
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "User email (will be lowercased + SHA256 hashed before sending)")
	cmd.Flags().StringVar(&value, "value", "", "Conversion value (string decimal)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code (used with --value)")
	cmd.Flags().StringVar(&occurredAt, "occurred-at", "", "Event timestamp YYYY-MM-DD or RFC3339; defaults to now")
	cmd.Flags().StringVar(&eventID, "event-id", "", "Caller-provided idempotency key")
	return cmd
}

func newConversionsTrackBatchCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "track-batch <rule-id>",
		Short: "Upload offline conversion events from a CSV file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			if file == "" {
				return errors.New("--file required")
			}
			records, err := readConversionCSV(file)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				return errors.New("CSV is empty (after header)")
			}
			// Build the first event so we can preview it via executeWrite.
			previewIn, err := buildConversionEvent(args[0], records[0])
			if err != nil {
				return fmt.Errorf("row 1: %w", err)
			}
			summary := fmt.Sprintf("POST /conversionEvents × %d", len(records))
			return executeWrite(cmd, summary, previewIn, func() error {
				return runConversionBatch(cmd, c, args[0], records)
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Path to CSV (header: email,occurred_at,value,currency,event_id)")
	return cmd
}

// runConversionBatch sends each record sequentially, printing progress to
// stderr and continuing on per-row failures. Returns nil even on partial
// failures so executeWrite still emits success markers — we summarise the
// failure count instead.
func runConversionBatch(cmd *cobra.Command, c *client.Client, ruleID string, records []conversionEventRecord) error {
	total := len(records)
	sent := 0
	failed := 0
	for i, rec := range records {
		in, err := buildConversionEvent(ruleID, rec)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "row %d: build: %v\n", i+1, err)
			failed++
			continue
		}
		if _, err := api.PostConversionEvent(cmd.Context(), c, in); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "row %d: send: %v\n", i+1, decorateConversionsAPIErr(err))
			failed++
			continue
		}
		sent++
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "sent %d/%d (%d errors)\n", sent, total, failed)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Done. Sent %d, failed %d.\n", sent, failed)
	if failed > 0 && sent == 0 {
		return fmt.Errorf("all %d events failed", total)
	}
	return nil
}

// buildConversionEvent assembles a ConversionEventInput from a record. The
// rule id is wrapped as urn:lla:llaPartnerConversion:<id> per the Conversions
// API contract. Email is lowercased and SHA256-hashed before sending.
func buildConversionEvent(ruleID string, rec conversionEventRecord) (*api.ConversionEventInput, error) {
	if rec.Email == "" {
		return nil, errors.New("email required")
	}
	in := &api.ConversionEventInput{
		Conversion:           "urn:lla:llaPartnerConversion:" + ruleID,
		ConversionHappenedAt: rec.OccurredAt.UnixMilli(),
		User: api.ConversionEventUser{
			UserIDs: []api.ConversionUserID{
				{IDType: "SHA256_EMAIL", IDValue: hashSHA256Email(rec.Email)},
			},
		},
		EventID: rec.EventID,
	}
	if rec.Value != "" {
		curr := rec.Currency
		if curr == "" {
			curr = "USD"
		}
		in.ConversionValue = &api.ConversionEventValue{CurrencyCode: curr, Amount: rec.Value}
	}
	return in, nil
}

// hashSHA256Email lowercases, trims and SHA256-hashes an email — the canonical
// form LinkedIn (and most matching networks) expect for SHA256_EMAIL ids.
func hashSHA256Email(email string) string {
	canon := strings.TrimSpace(strings.ToLower(email))
	sum := sha256.Sum256([]byte(canon))
	return hex.EncodeToString(sum[:])
}

// parseConversionTime accepts YYYY-MM-DD, RFC3339, or empty (now).
func parseConversionTime(s string) (time.Time, error) {
	if s == "" {
		return time.Now().UTC(), nil
	}
	if t, err := time.Parse(dateLayout, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time %q (want YYYY-MM-DD or RFC3339)", s)
}

// readConversionCSV reads and parses a CSV with header: email, occurred_at,
// value, currency, event_id. Missing optional columns are tolerated when the
// header doesn't include them.
func readConversionCSV(path string) ([]conversionEventRecord, error) {
	f, err := os.Open(path) //nolint:gosec // path supplied by user
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("CSV has no rows")
	}
	header := rows[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"email", "occurred_at"}
	for _, k := range required {
		if _, ok := idx[k]; !ok {
			return nil, fmt.Errorf("CSV missing required column %q", k)
		}
	}
	out := make([]conversionEventRecord, 0, len(rows)-1)
	for i, row := range rows[1:] {
		rec := conversionEventRecord{
			Email: strings.TrimSpace(row[idx["email"]]),
		}
		if v, ok := pickCSV(idx, row, "occurred_at"); ok {
			t, err := parseConversionTime(v)
			if err != nil {
				return nil, fmt.Errorf("row %d: %w", i+2, err)
			}
			rec.OccurredAt = t
		}
		if v, ok := pickCSV(idx, row, "value"); ok {
			rec.Value = v
		}
		if v, ok := pickCSV(idx, row, "currency"); ok {
			rec.Currency = v
		}
		if v, ok := pickCSV(idx, row, "event_id"); ok {
			rec.EventID = v
		}
		out = append(out, rec)
	}
	return out, nil
}

func pickCSV(idx map[string]int, row []string, key string) (string, bool) {
	i, ok := idx[key]
	if !ok || i >= len(row) {
		return "", false
	}
	v := strings.TrimSpace(row[i])
	if v == "" {
		return "", false
	}
	return v, true
}

// decorateConversionsAPIErr wraps a 403 from /conversionEvents with a hint to
// request the Conversions API product in the LinkedIn developer portal.
func decorateConversionsAPIErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "403") || strings.Contains(strings.ToLower(msg), "forbidden") {
		return fmt.Errorf("%w (request 'Conversions API' in your LinkedIn Developer Portal app — the token lacks that product)", err)
	}
	return err
}

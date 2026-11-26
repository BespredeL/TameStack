package dns

import (
	"strings"
	"testing"
)

func TestDNSRecordValidationAndRoundTrip(t *testing.T) {
	recs := []DNSRecord{
		{Name: "@", TTL: 3600, Type: "A", Value: "192.0.2.10"},
		{Name: "@", TTL: 3600, Type: "MX", Priority: 10, Value: "mail.example.com."},
		{Name: "@", TTL: 3600, Type: "TXT", Value: "v=spf1 mx a ~all"},
	}
	for _, r := range recs {
		if err := ValidateRecord(r); err != nil {
			t.Fatalf("ValidateRecord(%+v) unexpected error: %v", r, err)
		}
	}

	if err := ValidateRecord(DNSRecord{Name: "@", Type: "A", Value: "not-an-ip"}); err == nil {
		t.Errorf("expected error for invalid IPv4 in A record")
	}
	if err := ValidateRecord(DNSRecord{Name: "@;id", Type: "A", Value: "192.0.2.10"}); err == nil {
		t.Errorf("expected error for invalid record name")
	}

	zoneText := RenderZoneFile("example.com", 2026112601, recs)
	if !strings.Contains(zoneText, "2026112601") || !strings.Contains(zoneText, "192.0.2.10") {
		t.Errorf("unexpected zone file:\n%s", zoneText)
	}

	parsed := ParseZoneRecords(zoneText)
	if len(parsed) != 3 {
		t.Fatalf("ParseZoneRecords got %d records, want 3", len(parsed))
	}
	if parsed[1].Type != "MX" || parsed[1].Priority != 10 {
		t.Errorf("unexpected parsed MX record: %+v", parsed[1])
	}
}

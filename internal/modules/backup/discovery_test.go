package backup

import (
	"strings"
	"testing"
	"time"
)

func TestParseArchiveName(t *testing.T) {
	cases := []struct {
		name    string
		wantOK  bool
		typ     string
		target  string
		created time.Time
	}{
		{"site_example.com_20251010_121314.tar.gz", true, "site", "example.com",
			time.Date(2025, 10, 10, 12, 13, 14, 0, time.UTC)},
		{"db_my_db_20251010_121314.sql.gz", true, "database", "my_db",
			time.Date(2025, 10, 10, 12, 13, 14, 0, time.UTC)},
		{"configs_20251010_121314.tar.gz", true, "config", "",
			time.Date(2025, 10, 10, 12, 13, 14, 0, time.UTC)},
		{"full_backup_20251010_121314.tar.gz", true, "full", "",
			time.Date(2025, 10, 10, 12, 13, 14, 0, time.UTC)},
		{"site_a_20250101_010101.zip", false, "", "", time.Time{}},
		{"backup.tar.gz", false, "", "", time.Time{}},
		{"all_databases_20251010_121314.sql", false, "", "", time.Time{}},
		{"site_x_20251399_010101.tar.gz", false, "", "", time.Time{}},
		{"site_x_20251010_121314.tar.gz.tmp", false, "", "", time.Time{}},
	}
	for _, c := range cases {
		got, ok := parseArchiveName(c.name)
		if ok != c.wantOK {
			t.Errorf("parseArchiveName(%q) ok = %v, want %v", c.name, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if got.BackupType != c.typ || got.TargetName != c.target || !got.CreatedAt.Equal(c.created) {
			t.Errorf("parseArchiveName(%q) = %+v; want type=%q target=%q created=%v",
				c.name, got, c.typ, c.target, c.created)
		}
	}
}

func TestParseFindOutputReconciles(t *testing.T) {
	output := strings.Join([]string{
		"111 /var/backups/TameStack/site_example.com_20251010_121314.tar.gz",
		"222 /var/backups/TameStack/db_my_db_20251011_010203.sql.gz",
		"333 /var/backups/TameStack/configs_20251012_101112.tar.gz",
		"444 /var/backups/TameStack/full_backup_20251013_151617.tar.gz",
		"555 /var/backups/TameStack/already_known.tar.gz",
		"666 /var/backups/TameStack/manual_snapshot_2025-10-09.tar.gz",
		"not-a-line",
		"",
	}, "\n")

	known := map[string]bool{"/var/backups/TameStack/already_known.tar.gz": true}
	records, alreadyKnown, unknown := parseFindOutput(output, "srv-1", known)

	if alreadyKnown != 1 {
		t.Errorf("alreadyKnown = %d, want 1", alreadyKnown)
	}
	if unknown != 1 { // manual_snapshot only; the malformed line is skipped entirely
		t.Errorf("unknown = %d, want 1", unknown)
	}
	if len(records) != 4 {
		t.Fatalf("registered candidates = %d, want 4: %+v", len(records), records)
	}

	wantTypes := []string{"site", "database", "config", "full"}
	wantTargets := []string{"example.com", "my_db", "", ""}
	wantSizes := []int64{111, 222, 333, 444}
	for i, rec := range records {
		if rec.BackupType != wantTypes[i] || rec.TargetName != wantTargets[i] || rec.SizeBytes != wantSizes[i] {
			t.Errorf("record %d = %+v, want type=%s target=%s size=%d",
				i, rec, wantTypes[i], wantTargets[i], wantSizes[i])
		}
		if rec.ServerID != "srv-1" || rec.Status != "completed" || rec.Verified || rec.Offsite {
			t.Errorf("record %d has wrong defaults: %+v", i, rec)
		}
		if rec.ID == "" {
			t.Errorf("record %d missing id", i)
		}
	}
	if records[0].CreatedAt.UTC().Format("2006-01-02") != "2025-10-10" {
		t.Errorf("site record created_at = %v", records[0].CreatedAt)
	}
}

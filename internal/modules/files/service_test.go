package files

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		input    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, c := range cases {
		out := formatBytes(c.input)
		if out != c.expected {
			t.Errorf("formatBytes(%d): expected %s, got %s", c.input, c.expected, out)
		}
	}
}

func TestParseByteSizeUnit(t *testing.T) {
	cases := []struct {
		input    string
		expected int64
	}{
		{"100B", 100},
		{"1.5KB", 1536},
		{"10M", 10 * 1024 * 1024},
		{"1.2G", 1288490188},
		{"2.0 GB", 2 * 1024 * 1024 * 1024},
		{"", 0},
	}

	for _, c := range cases {
		out := parseByteSizeUnit(c.input)
		if out != c.expected {
			t.Errorf("parseByteSizeUnit(%q): expected %d, got %d", c.input, c.expected, out)
		}
	}
}

func TestParseDFPartitions(t *testing.T) {
	dfSample := `Filesystem     1B-blocks        Used   Available Use% Mounted on
udev          2075779072           0  2075779072   0% /dev
/dev/sda1    42199040000 15204352000 24844352000  38% /
/dev/sdb1   107374182400 53687091200 48318357504  53% /mnt/data
`
	partitions := parseDFPartitions(dfSample)
	if len(partitions) != 3 {
		t.Fatalf("expected 3 partitions, got %d", len(partitions))
	}

	root := partitions[1]
	if root.MountPoint != "/" {
		t.Errorf("expected mount /, got %s", root.MountPoint)
	}
	if root.TotalBytes != 42199040000 {
		t.Errorf("expected TotalBytes 42199040000, got %d", root.TotalBytes)
	}
	if root.UsedPercent != 36.0 { // 15204352000 / 42199040000 = ~36.0%
		t.Errorf("expected UsedPercent ~36.0, got %f", root.UsedPercent)
	}
}

func TestParseDuOutput(t *testing.T) {
	duSample := `4096	/var/log/nginx
10240	/var/log/journal
2048	/var/log/syslog
16384	/var/log
`
	items, total := parseDuOutput(duSample, "/var/log")
	if total != 16384*1024 {
		t.Errorf("expected total %d, got %d", 16384*1024, total)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	// Sorted descending: journal (10MB), nginx (4MB), syslog (2MB)
	if items[0].Name != "journal" {
		t.Errorf("expected first item 'journal', got %s", items[0].Name)
	}
	if items[0].Percent < 60.0 {
		t.Errorf("expected percent > 60%%, got %f", items[0].Percent)
	}
}

func TestParseFindLargestFiles(t *testing.T) {
	sample := `52428800|1700000000.5|/var/log/nginx/access.log
10485760|1700000001.0|/var/log/syslog
`
	items := parseFindLargestFiles(sample)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "access.log" {
		t.Errorf("expected access.log, got %s", items[0].Name)
	}
	if items[0].SizeBytes != 52428800 {
		t.Errorf("expected 52428800 bytes, got %d", items[0].SizeBytes)
	}
}

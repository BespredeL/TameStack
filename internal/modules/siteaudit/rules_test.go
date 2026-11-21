package siteaudit

import "testing"

func TestParseAndCompare(t *testing.T) {
	raw := "index.php\t10\t644\twww-data\t1\taaa\nuploads/x.php\t4\t644\twww-data\t2\tbbb\n../etc/passwd\t1\t644\troot\t3\tccc\n"
	files := ParseManifest(raw)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %#v", files)
	}
	base := []FileEntry{{Path: "index.php", SHA256: "aaa"}}
	diff := CompareManifests(base, files)
	if len(diff.Added) != 1 || diff.Added[0] != "uploads/x.php" {
		t.Fatalf("added: %#v", diff.Added)
	}
	findings := EvaluateRules(files, diff, map[string]string{
		"uploads/x.php": "<?php eval(base64_decode('abc'));",
	})
	if len(findings) < 2 {
		t.Fatalf("expected upload and eval findings, got %#v", findings)
	}
}

func TestCleanIndexIsNotAFinding(t *testing.T) {
	files := []FileEntry{{Path: "index.php", SHA256: "aaa"}}
	findings := EvaluateRules(files, Diff{}, nil)
	if len(findings) != 0 {
		t.Fatalf("unchanged index.php must not be flagged: %#v", findings)
	}
}

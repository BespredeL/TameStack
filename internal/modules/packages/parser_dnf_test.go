package packages

import "testing"

const dnfInstallOutput = `Last metadata expiration check: 0:01:02 ago on Mon 17 Nov 2026 10:00:00 AM UTC.
Dependencies resolved.
================================================================================
 Package            Arch        Version               Repository          Size
================================================================================
Installing:
 nginx              x86_64      1:1.24.0-1.el9        appstream          500 k
Installing dependencies:
 nginx-core         x86_64      1:1.24.0-1.el9        appstream          400 k
Upgrading:
 openssl            x86_64      1:3.0.7-2.el9         baseos             1.2 M

Transaction Summary
================================================================================
Install  2 Packages
Upgrade  1 Package

Total download size: 2.1 M
Installed size: 8.0 M
Is this ok [y/N]: Operation aborted.
`

const dnfRemoveOutput = `Dependencies resolved.
================================================================================
 Package         Arch        Version              Repository           Size
================================================================================
Removing:
 nginx           x86_64      1:1.24.0-1.el9       @appstream           500 k
Removing dependent packages:
 nginx-core      x86_64      1:1.24.0-1.el9       @appstream           400 k

Transaction Summary
================================================================================
Remove  2 Packages

Freed space: 2.0 M
Is this ok [y/N]: Operation aborted.
`

func TestParseDnfSimulationInstall(t *testing.T) {
	res, err := ParseDnfSimulation(dnfInstallOutput, "dnf install --assumeno nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatal("expected Success=true")
	}
	if res.InstalledCount != 2 || res.UpgradedCount != 1 || res.RemovedCount != 0 {
		t.Errorf("counts = install %d upgrade %d remove %d, want 2/1/0", res.InstalledCount, res.UpgradedCount, res.RemovedCount)
	}
	if len(res.Changes) != 3 {
		t.Fatalf("changes = %d, want 3", len(res.Changes))
	}
	byName := map[string]PackageChange{}
	for _, c := range res.Changes {
		byName[c.Name] = c
	}
	nginx := byName["nginx"]
	if nginx.Action != "install" || nginx.Arch != "x86_64" || nginx.NewVersion != "1:1.24.0-1.el9" {
		t.Errorf("nginx change = %+v", nginx)
	}
	if byName["nginx-core"].Action != "install" {
		t.Errorf("nginx-core action = %q, want install", byName["nginx-core"].Action)
	}
	openssl := byName["openssl"]
	if openssl.Action != "upgrade" || openssl.NewVersion != "1:3.0.7-2.el9" {
		t.Errorf("openssl change = %+v", openssl)
	}
}

func TestParseDnfSimulationRemove(t *testing.T) {
	res, err := ParseDnfSimulation(dnfRemoveOutput, "dnf remove --assumeno nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RemovedCount != 2 {
		t.Errorf("RemovedCount = %d, want 2", res.RemovedCount)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(res.Changes))
	}
	for _, c := range res.Changes {
		if c.Action != "remove" {
			t.Errorf("%s action = %q, want remove", c.Name, c.Action)
		}
		if c.OldVersion != "1:1.24.0-1.el9" {
			t.Errorf("%s OldVersion = %q", c.Name, c.OldVersion)
		}
	}
}

func TestParseDnfSimulationEmpty(t *testing.T) {
	res, err := ParseDnfSimulation("", "dnf upgrade --assumeno")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Changes) != 0 || res.InstalledCount != 0 || res.UpgradedCount != 0 {
		t.Errorf("expected empty result, got %+v", res)
	}
}

func TestParseRpmList(t *testing.T) {
	out := "nginx\t1:1.24.0-1.el9\tx86_64\tHigh performance web server\n" +
		"openssl\t1:3.0.7-2.el9\tx86_64\tCryptography library\n" +
		"bash\t5.1.8-6.el9\tx86_64\tThe GNU Bourne Again shell\n"
	items, err := ParseRpmList(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	if items[0].Name != "nginx" || items[0].Version != "1:1.24.0-1.el9" || items[0].Architecture != "x86_64" {
		t.Errorf("first item = %+v", items[0])
	}
	if items[0].Status != "installed" || items[0].Description != "High performance web server" {
		t.Errorf("first item status/desc = %q / %q", items[0].Status, items[0].Description)
	}
}

func TestParseRpmListSkipsShortLines(t *testing.T) {
	items, err := ParseRpmList("bad-line\nnginx\t1.0\tx86_64\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "nginx" {
		t.Errorf("items = %+v, want single nginx", items)
	}
}
